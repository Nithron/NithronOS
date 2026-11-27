package net

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	// firewallStatePath is the confirmed rule set (mode and rules); the
	// agent keeps the loaded script in /etc/nftables.d/nos.nft for boot.
	firewallStatePath  = "/etc/nos/firewall-state.json"
	nftablesBackupPath = "/etc/nos/firewall.backup.nft"
	rollbackTimeout    = 60 * time.Second
)

// FirewallManager manages nftables firewall rules
type FirewallManager struct {
	mu             sync.RWMutex
	currentState   *FirewallState
	pendingPlan    *FirewallPlan
	rollbackTimer  *time.Timer
	rollbackCancel chan struct{}
	configPath     string
	backupPath     string
	nft            NFTables
	wgPort         int
	pendingScript  string // applied, not yet confirmed
}

// NFTables reads and replaces the kernel's ruleset. nosd runs as the nos
// user, which can do neither, so the server gives the manager one backed by
// nos-agent (SetNFTables); the default runs nft directly (tests, root).
type NFTables interface {
	// List returns `nft list ruleset` (json selects -j).
	List(json bool) ([]byte, error)
	// Apply checks and loads script; persist keeps it across reboots.
	Apply(script string, persist bool) error
}

type execNFTables struct{}

func (execNFTables) List(json bool) ([]byte, error) {
	if json {
		return exec.Command("nft", "-j", "list", "ruleset").Output()
	}
	return exec.Command("nft", "list", "ruleset").Output()
}

func (execNFTables) Apply(script string, _ bool) error {
	tmp, err := os.CreateTemp("", "nft-*.rules")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	if out, err := exec.Command("nft", "-f", tmp.Name()).CombinedOutput(); err != nil {
		return fmt.Errorf("nft failed: %s: %w", out, err)
	}
	return nil
}

// NewFirewallManager creates a new firewall manager
func NewFirewallManager() *FirewallManager {
	return &FirewallManager{
		configPath: firewallStatePath,
		backupPath: nftablesBackupPath,
		nft:        execNFTables{},
	}
}

// SetNFTables replaces how the manager reads and loads rulesets.
func (fm *FirewallManager) SetNFTables(n NFTables) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.nft = n
}

// GetState returns the current firewall state
func (fm *FirewallManager) GetState() (*FirewallState, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.currentState == nil {
		// Load current state from nftables
		state, err := fm.loadCurrentState()
		if err != nil {
			return nil, fmt.Errorf("failed to load firewall state: %w", err)
		}
		fm.currentState = state
	}

	return fm.currentState, nil
}

// CreatePlan creates a firewall configuration plan
func (fm *FirewallManager) CreatePlan(mode AccessMode, enableWG, enableHTTPS bool, customRules []FirewallRule) (*FirewallPlan, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	currentState, err := fm.loadCurrentState()
	if err != nil {
		return nil, fmt.Errorf("failed to load current state: %w", err)
	}

	desiredState := fm.generateDesiredState(mode, enableWG, enableHTTPS, customRules)
	changes := fm.calculateDiff(currentState, desiredState)

	// Generate dry run output
	dryRunOutput, err := fm.generateNFTablesScript(desiredState, true)
	if err != nil {
		return nil, fmt.Errorf("failed to generate dry run: %w", err)
	}

	plan := &FirewallPlan{
		ID:           generateID(),
		CurrentState: currentState,
		DesiredState: desiredState,
		Changes:      changes,
		DryRunOutput: dryRunOutput,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}

	fm.pendingPlan = plan
	return plan, nil
}

// ApplyPlan applies a firewall plan with automatic rollback
func (fm *FirewallManager) ApplyPlan(planID string, rollbackTimeoutSec int) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.pendingPlan == nil || fm.pendingPlan.ID != planID {
		return fmt.Errorf("plan not found or expired")
	}

	if time.Now().After(fm.pendingPlan.ExpiresAt) {
		return fmt.Errorf("plan has expired")
	}

	// Backup current configuration
	if err := fm.backupCurrentConfig(); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}

	// Generate and apply new configuration
	script, err := fm.generateNFTablesScript(fm.pendingPlan.DesiredState, false)
	if err != nil {
		return fmt.Errorf("failed to generate script: %w", err)
	}

	if err := fm.applyNFTablesScript(script); err != nil {
		// Rollback on failure
		if rbErr := fm.rollbackToBackup(); rbErr != nil {
			// Log rollback error but return original error
			fmt.Printf("Failed to rollback firewall: %v\n", rbErr)
		}
		return fmt.Errorf("failed to apply firewall rules: %w", err)
	}

	// Update state
	fm.currentState = fm.pendingPlan.DesiredState
	fm.currentState.Status = "pending_confirm"
	fm.currentState.LastApplied = time.Now()

	// Set rollback timer
	timeout := time.Duration(rollbackTimeoutSec) * time.Second
	if timeout == 0 {
		timeout = rollbackTimeout
	}

	rollbackAt := time.Now().Add(timeout)
	fm.currentState.RollbackAt = &rollbackAt

	fm.rollbackCancel = make(chan struct{})
	fm.rollbackTimer = time.AfterFunc(timeout, func() {
		fm.autoRollback()
	})

	return nil
}

// ConfirmPlan confirms the applied firewall configuration
func (fm *FirewallManager) ConfirmPlan() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.currentState == nil || fm.currentState.Status != "pending_confirm" {
		return fmt.Errorf("no pending configuration to confirm")
	}

	// Cancel rollback timer
	if fm.rollbackTimer != nil {
		fm.rollbackTimer.Stop()
		close(fm.rollbackCancel)
		fm.rollbackTimer = nil
		fm.rollbackCancel = nil
	}

	// Keep the confirmed rules for the next boot, and remember them.
	if fm.pendingScript != "" {
		if err := fm.nft.Apply(fm.pendingScript, true); err != nil {
			return fmt.Errorf("failed to keep the rules: %w", err)
		}
	}
	fm.currentState.Status = "active"
	fm.currentState.RollbackAt = nil
	if data, err := json.MarshalIndent(fm.currentState, "", "  "); err == nil {
		if err := os.WriteFile(fm.configPath, data, 0o600); err != nil {
			return fmt.Errorf("failed to save the firewall state: %w", err)
		}
	}
	fm.pendingPlan = nil
	fm.pendingScript = ""
	os.Remove(fm.backupPath)
	return nil
}

// Rollback manually triggers a rollback to the previous configuration
func (fm *FirewallManager) Rollback() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	return fm.rollbackToBackup()
}

// Private methods

// loadCurrentState is the last confirmed rule set, or the LAN defaults.
// (It used to claim to parse the live ruleset and return the defaults.)
func (fm *FirewallManager) loadCurrentState() (*FirewallState, error) {
	var st FirewallState
	data, err := os.ReadFile(fm.configPath)
	if err == nil && json.Unmarshal(data, &st) == nil && len(st.Rules) > 0 {
		return &st, nil
	}
	rules := BaseRules()
	return &FirewallState{
		Mode:     AccessModeLANOnly,
		Rules:    rules,
		Checksum: fm.calculateRulesChecksum(rules),
		Status:   "default",
	}, nil
}

// SetWireGuardPort is the UDP port WireGuard listens on (51820 by default).
func (fm *FirewallManager) SetWireGuardPort(port int) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.wgPort = port
}

// generateDesiredState is the LAN base plus what the mode opens. (WireGuard
// and public HTTPS used to replace the LAN rules, which would have locked
// the LAN out of the web interface, and no mode allowed SMB or NFS.)
func (fm *FirewallManager) generateDesiredState(mode AccessMode, enableWG, enableHTTPS bool, customRules []FirewallRule) *FirewallState {
	rules := BaseRules()
	switch mode {
	case AccessModeWireGuard:
		if enableWG {
			rules = append(rules, WireGuardRules(fm.wgPort, "wg0")...)
		}
	case AccessModePublicHTTPS:
		if enableHTTPS {
			rules = append(rules, PublicHTTPSRules()...)
		}
	}
	rules = append(rules, customRules...)
	return &FirewallState{
		Mode:     mode,
		Rules:    rules,
		Checksum: fm.calculateRulesChecksum(rules),
		Status:   "planned",
	}
}

func (fm *FirewallManager) calculateDiff(current, desired *FirewallState) []FirewallDiff {
	var diffs []FirewallDiff

	// Create maps for easy lookup
	currentRules := make(map[string]*FirewallRule)
	for i := range current.Rules {
		currentRules[current.Rules[i].ID] = &current.Rules[i]
	}

	desiredRules := make(map[string]*FirewallRule)
	for i := range desired.Rules {
		desiredRules[desired.Rules[i].ID] = &desired.Rules[i]
	}

	// Find removed rules
	for id, rule := range currentRules {
		if _, exists := desiredRules[id]; !exists {
			diffs = append(diffs, FirewallDiff{
				Type:        "remove",
				OldRule:     rule,
				Description: fmt.Sprintf("Remove rule: %s", rule.Description),
			})
		}
	}

	// Find added and modified rules
	for id, rule := range desiredRules {
		if oldRule, exists := currentRules[id]; exists {
			// Check if modified
			if !rulesEqual(oldRule, rule) {
				diffs = append(diffs, FirewallDiff{
					Type:        "modify",
					Rule:        rule,
					OldRule:     oldRule,
					Description: fmt.Sprintf("Modify rule: %s", rule.Description),
				})
			}
		} else {
			// New rule
			diffs = append(diffs, FirewallDiff{
				Type:        "add",
				Rule:        rule,
				Description: fmt.Sprintf("Add rule: %s", rule.Description),
			})
		}
	}

	return diffs
}

func (fm *FirewallManager) generateNFTablesScript(state *FirewallState, _ bool) (string, error) {
	return RenderScript(state.Rules)
}

// applyNFTablesScript loads script now but doesn't keep it for the next
// boot: that happens when the plan is confirmed, so a reboot inside the
// confirm window comes back with the previous rules.
func (fm *FirewallManager) applyNFTablesScript(script string) error {
	if err := fm.nft.Apply(script, false); err != nil {
		return err
	}
	fm.pendingScript = script
	return nil
}

// backupCurrentConfig saves table inet nos as it is now (empty when there
// is none yet).
func (fm *FirewallManager) backupCurrentConfig() error {
	output, err := fm.nft.List(false)
	if err != nil {
		output = nil // no nos table yet
	}
	if err := os.WriteFile(fm.backupPath, output, 0600); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}
	return nil
}

// rollbackToBackup puts table inet nos back as it was before the apply;
// nothing else is touched (it used to flush the whole ruleset, Docker's
// included). What boot loads was never changed, so it isn't either.
func (fm *FirewallManager) rollbackToBackup() error {
	backup, err := os.ReadFile(fm.backupPath)
	if err != nil {
		return fmt.Errorf("no backup found")
	}
	script := fmt.Sprintf("table inet %s\ndelete table inet %s\n%s", NFTTable, NFTTable, backup)
	if err := fm.nft.Apply(script, false); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	fm.pendingScript = ""
	if fm.currentState != nil {
		fm.currentState.Status = "active"
		fm.currentState.RollbackAt = nil
	}
	if fm.rollbackTimer != nil {
		fm.rollbackTimer.Stop()
		if fm.rollbackCancel != nil {
			close(fm.rollbackCancel)
		}
		fm.rollbackTimer = nil
		fm.rollbackCancel = nil
	}
	return nil
}

func (fm *FirewallManager) autoRollback() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.currentState != nil && fm.currentState.Status == "pending_confirm" {
		fm.currentState.Status = "rolling_back"
		if err := fm.rollbackToBackup(); err != nil {
			fmt.Printf("Failed to rollback during auto-rollback: %v\n", err)
		}
	}
}

// Helper functions

func (fm *FirewallManager) calculateRulesChecksum(rules []FirewallRule) string {
	var data bytes.Buffer
	for _, rule := range rules {
		fmt.Fprintf(&data, "%v", rule)
	}
	hash := sha256.Sum256(data.Bytes())
	return hex.EncodeToString(hash[:])
}

func rulesEqual(a, b *FirewallRule) bool {
	return a.ID == b.ID &&
		a.Table == b.Table &&
		a.Chain == b.Chain &&
		a.Priority == b.Priority &&
		a.Type == b.Type &&
		a.Protocol == b.Protocol &&
		a.SourceCIDR == b.SourceCIDR &&
		a.DestPort == b.DestPort &&
		a.Action == b.Action &&
		a.Enabled == b.Enabled
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
