package net

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	nftablesConfigPath = "/etc/nos/firewall.nft"
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
}

// NewFirewallManager creates a new firewall manager
func NewFirewallManager() *FirewallManager {
	return &FirewallManager{
		configPath: nftablesConfigPath,
		backupPath: nftablesBackupPath,
	}
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

	// Update state
	fm.currentState.Status = "active"
	fm.currentState.RollbackAt = nil

	// Clear pending plan
	fm.pendingPlan = nil

	// Remove backup as configuration is confirmed
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

func (fm *FirewallManager) loadCurrentState() (*FirewallState, error) {
	// Parse current nftables rules
	cmd := exec.Command("nft", "-j", "list", "ruleset")
	output, err := cmd.Output()
	if err != nil {
		// If nftables is empty, return default state
		return &FirewallState{
			Mode:        AccessModeLANOnly,
			Rules:       fm.getDefaultRules(),
			LastApplied: time.Now(),
			Status:      "active",
		}, nil
	}

	// Parse JSON output and convert to FirewallState
	// For now, return a simplified state
	return &FirewallState{
		Mode:        fm.detectCurrentMode(output),
		Rules:       fm.parseNFTablesRules(output),
		LastApplied: time.Now(),
		Checksum:    fm.calculateChecksum(output),
		Status:      "active",
	}, nil
}

func (fm *FirewallManager) generateDesiredState(mode AccessMode, enableWG, enableHTTPS bool, customRules []FirewallRule) *FirewallState {
	rules := fm.getDefaultRules()

	// Add mode-specific rules
	switch mode {
	case AccessModeLANOnly:
		rules = append(rules, fm.getLANOnlyRules()...)
	case AccessModeWireGuard:
		if enableWG {
			rules = append(rules, fm.getWireGuardRules()...)
		}
	case AccessModePublicHTTPS:
		if enableHTTPS {
			rules = append(rules, fm.getPublicHTTPSRules()...)
		}
	}

	// Add custom rules
	rules = append(rules, customRules...)

	return &FirewallState{
		Mode:     mode,
		Rules:    rules,
		Checksum: fm.calculateRulesChecksum(rules),
		Status:   "planned",
	}
}

func (fm *FirewallManager) getDefaultRules() []FirewallRule {
	return []FirewallRule{
		{
			ID:          "allow-loopback",
			Table:       "filter",
			Chain:       "input",
			Priority:    10,
			Type:        "allow",
			SourceCIDR:  "127.0.0.0/8",
			Action:      "accept",
			Description: "Allow loopback traffic",
			Enabled:     true,
		},
		{
			ID:          "allow-established",
			Table:       "filter",
			Chain:       "input",
			Priority:    20,
			Type:        "allow",
			Action:      "accept",
			Description: "Allow established connections",
			Enabled:     true,
		},
		{
			ID:          "allow-icmp",
			Table:       "filter",
			Chain:       "input",
			Priority:    30,
			Type:        "allow",
			Protocol:    "icmp",
			Action:      "accept",
			Description: "Allow ICMP",
			Enabled:     true,
		},
	}
}

func (fm *FirewallManager) getLANOnlyRules() []FirewallRule {
	return []FirewallRule{
		{
			ID:          "allow-lan-ssh",
			Table:       "filter",
			Chain:       "input",
			Priority:    100,
			Type:        "allow",
			Protocol:    "tcp",
			SourceCIDR:  "192.168.0.0/16",
			DestPort:    "22",
			Action:      "accept",
			Description: "Allow SSH from LAN",
			Enabled:     true,
		},
		{
			ID:          "allow-lan-http",
			Table:       "filter",
			Chain:       "input",
			Priority:    101,
			Type:        "allow",
			Protocol:    "tcp",
			SourceCIDR:  "192.168.0.0/16",
			DestPort:    "80",
			Action:      "accept",
			Description: "Allow HTTP from LAN",
			Enabled:     true,
		},
		{
			ID:          "allow-lan-https",
			Table:       "filter",
			Chain:       "input",
			Priority:    102,
			Type:        "allow",
			Protocol:    "tcp",
			SourceCIDR:  "192.168.0.0/16",
			DestPort:    "443",
			Action:      "accept",
			Description: "Allow HTTPS from LAN",
			Enabled:     true,
		},
		{
			ID:          "block-wan-http",
			Table:       "filter",
			Chain:       "input",
			Priority:    200,
			Type:        "deny",
			Protocol:    "tcp",
			DestPort:    "80,443",
			Action:      "drop",
			Description: "Block HTTP/HTTPS from WAN",
			Enabled:     true,
		},
	}
}

func (fm *FirewallManager) getWireGuardRules() []FirewallRule {
	return []FirewallRule{
		{
			ID:          "allow-wireguard",
			Table:       "filter",
			Chain:       "input",
			Priority:    50,
			Type:        "allow",
			Protocol:    "udp",
			DestPort:    "51820",
			Action:      "accept",
			Description: "Allow WireGuard VPN",
			Enabled:     true,
		},
	}
}

func (fm *FirewallManager) getPublicHTTPSRules() []FirewallRule {
	return []FirewallRule{
		{
			ID:          "allow-http-acme",
			Table:       "filter",
			Chain:       "input",
			Priority:    40,
			Type:        "allow",
			Protocol:    "tcp",
			DestPort:    "80",
			Action:      "accept",
			Description: "Allow HTTP for ACME challenges",
			Enabled:     true,
		},
		{
			ID:          "allow-https-public",
			Table:       "filter",
			Chain:       "input",
			Priority:    41,
			Type:        "allow",
			Protocol:    "tcp",
			DestPort:    "443",
			Action:      "accept",
			Description: "Allow HTTPS from public",
			Enabled:     true,
		},
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

func (fm *FirewallManager) generateNFTablesScript(state *FirewallState, dryRun bool) (string, error) {
	var buf bytes.Buffer

	// Start with flush
	if !dryRun {
		buf.WriteString("#!/usr/sbin/nft -f\n")
		buf.WriteString("flush ruleset\n\n")
	}

	// Create base table and chains
	buf.WriteString("table inet filter {\n")
	buf.WriteString("    chain input {\n")
	buf.WriteString("        type filter hook input priority 0; policy drop;\n")

	// Add rules sorted by priority
	for _, rule := range state.Rules {
		if !rule.Enabled || rule.Chain != "input" {
			continue
		}

		buf.WriteString("        ")

		// Build rule string
		if rule.Protocol != "" {
			fmt.Fprintf(&buf, "ip protocol %s ", rule.Protocol)
		}
		if rule.SourceCIDR != "" && rule.SourceCIDR != "127.0.0.0/8" {
			fmt.Fprintf(&buf, "ip saddr %s ", rule.SourceCIDR)
		}
		if rule.DestPort != "" {
			fmt.Fprintf(&buf, "tcp dport { %s } ", rule.DestPort)
		}

		// Special handling for established connections
		if rule.ID == "allow-established" {
			buf.WriteString("ct state established,related ")
		}

		fmt.Fprintf(&buf, "%s comment \"%s\"\n", rule.Action, rule.Description)
	}

	buf.WriteString("    }\n")
	buf.WriteString("    chain forward {\n")
	buf.WriteString("        type filter hook forward priority 0; policy drop;\n")
	buf.WriteString("    }\n")
	buf.WriteString("    chain output {\n")
	buf.WriteString("        type filter hook output priority 0; policy accept;\n")
	buf.WriteString("    }\n")
	buf.WriteString("}\n")

	return buf.String(), nil
}

func (fm *FirewallManager) applyNFTablesScript(script string) error {
	// Write script to temporary file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("nft-%d.rules", time.Now().Unix()))
	if err := os.WriteFile(tmpFile, []byte(script), 0600); err != nil {
		return fmt.Errorf("failed to write script: %w", err)
	}
	defer os.Remove(tmpFile)

	// Apply the script
	cmd := exec.Command("nft", "-f", tmpFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft failed: %s: %w", output, err)
	}

	// Save to config path for persistence
	if err := os.WriteFile(fm.configPath, []byte(script), 0600); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func (fm *FirewallManager) backupCurrentConfig() error {
	// Get current ruleset
	cmd := exec.Command("nft", "list", "ruleset")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current ruleset: %w", err)
	}

	// Save to backup file
	if err := os.WriteFile(fm.backupPath, output, 0600); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

func (fm *FirewallManager) rollbackToBackup() error {
	// Check if backup exists
	if _, err := os.Stat(fm.backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}

	// Apply backup
	cmd := exec.Command("nft", "-f", fm.backupPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rollback failed: %s: %w", output, err)
	}

	// Update state
	if fm.currentState != nil {
		fm.currentState.Status = "active"
		fm.currentState.RollbackAt = nil
	}

	// Cancel rollback timer if exists
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

func (fm *FirewallManager) detectCurrentMode(nftOutput []byte) AccessMode {
	output := string(nftOutput)

	// Simple heuristic based on rules present
	if strings.Contains(output, "51820") {
		return AccessModeWireGuard
	}
	if strings.Contains(output, "dport 443") && !strings.Contains(output, "192.168") {
		return AccessModePublicHTTPS
	}

	return AccessModeLANOnly
}

func (fm *FirewallManager) parseNFTablesRules(output []byte) []FirewallRule {
	// This would parse the JSON output from nft
	// For now, return current rules based on detected mode
	return fm.getDefaultRules()
}

func (fm *FirewallManager) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

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
