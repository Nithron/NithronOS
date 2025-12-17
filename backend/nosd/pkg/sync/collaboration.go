// Package sync provides file synchronization functionality for NithronOS.
package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SharePermission represents the permission level for a shared folder
type SharePermission string

const (
	PermissionRead      SharePermission = "read"
	PermissionWrite     SharePermission = "write"
	PermissionAdmin     SharePermission = "admin"
)

// ShareInviteStatus represents the status of a share invitation
type ShareInviteStatus string

const (
	InvitePending  ShareInviteStatus = "pending"
	InviteAccepted ShareInviteStatus = "accepted"
	InviteDeclined ShareInviteStatus = "declined"
	InviteExpired  ShareInviteStatus = "expired"
)

// SharedFolder represents a folder shared between users
type SharedFolder struct {
	ID          string          `json:"id"`
	ShareID     string          `json:"share_id"`
	Path        string          `json:"path"`
	Name        string          `json:"name"`
	OwnerID     string          `json:"owner_id"`
	OwnerName   string          `json:"owner_name"`
	CreatedAt   time.Time       `json:"created_at"`
	Members     []FolderMember  `json:"members"`
}

// FolderMember represents a member with access to a shared folder
type FolderMember struct {
	UserID      string          `json:"user_id"`
	Username    string          `json:"username"`
	Permission  SharePermission `json:"permission"`
	AddedAt     time.Time       `json:"added_at"`
	AddedBy     string          `json:"added_by"`
}

// ShareInvite represents an invitation to join a shared folder
type ShareInvite struct {
	ID             string            `json:"id"`
	SharedFolderID string            `json:"shared_folder_id"`
	FolderName     string            `json:"folder_name"`
	InviterID      string            `json:"inviter_id"`
	InviterName    string            `json:"inviter_name"`
	InviteeID      string            `json:"invitee_id"`
	InviteeEmail   string            `json:"invitee_email,omitempty"`
	Permission     SharePermission   `json:"permission"`
	Status         ShareInviteStatus `json:"status"`
	Message        string            `json:"message,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	ExpiresAt      time.Time         `json:"expires_at"`
	RespondedAt    *time.Time        `json:"responded_at,omitempty"`
}

// CollaborationStore manages shared folders and invitations
type CollaborationStore struct {
	dataDir       string
	sharedFolders map[string]*SharedFolder
	invites       map[string]*ShareInvite
	mu            sync.RWMutex
}

// NewCollaborationStore creates a new collaboration store
func NewCollaborationStore(dataDir string) (*CollaborationStore, error) {
	store := &CollaborationStore{
		dataDir:       dataDir,
		sharedFolders: make(map[string]*SharedFolder),
		invites:       make(map[string]*ShareInvite),
	}

	// Create data directory if needed
	collabDir := filepath.Join(dataDir, "collaboration")
	if err := os.MkdirAll(collabDir, 0750); err != nil {
		return nil, err
	}

	// Load existing data
	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// CreateSharedFolder creates a new shared folder
func (s *CollaborationStore) CreateSharedFolder(shareID, path, name, ownerID, ownerName string) (*SharedFolder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder := &SharedFolder{
		ID:        uuid.New().String(),
		ShareID:   shareID,
		Path:      path,
		Name:      name,
		OwnerID:   ownerID,
		OwnerName: ownerName,
		CreatedAt: time.Now(),
		Members: []FolderMember{
			{
				UserID:     ownerID,
				Username:   ownerName,
				Permission: PermissionAdmin,
				AddedAt:    time.Now(),
				AddedBy:    ownerID,
			},
		},
	}

	s.sharedFolders[folder.ID] = folder

	if err := s.save(); err != nil {
		delete(s.sharedFolders, folder.ID)
		return nil, err
	}

	return folder, nil
}

// GetSharedFolder retrieves a shared folder by ID
func (s *CollaborationStore) GetSharedFolder(id string) (*SharedFolder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folder, ok := s.sharedFolders[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return folder, nil
}

// ListSharedFolders returns folders accessible to a user
func (s *CollaborationStore) ListSharedFolders(userID string) []*SharedFolder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SharedFolder
	for _, folder := range s.sharedFolders {
		for _, member := range folder.Members {
			if member.UserID == userID {
				result = append(result, folder)
				break
			}
		}
	}
	return result
}

// ListOwnedFolders returns folders owned by a user
func (s *CollaborationStore) ListOwnedFolders(userID string) []*SharedFolder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SharedFolder
	for _, folder := range s.sharedFolders {
		if folder.OwnerID == userID {
			result = append(result, folder)
		}
	}
	return result
}

// AddMember adds a member to a shared folder
func (s *CollaborationStore) AddMember(folderID, userID, username string, permission SharePermission, addedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder, ok := s.sharedFolders[folderID]
	if !ok {
		return os.ErrNotExist
	}

	// Check if already a member
	for _, m := range folder.Members {
		if m.UserID == userID {
			return nil // Already a member
		}
	}

	folder.Members = append(folder.Members, FolderMember{
		UserID:     userID,
		Username:   username,
		Permission: permission,
		AddedAt:    time.Now(),
		AddedBy:    addedBy,
	})

	return s.save()
}

// RemoveMember removes a member from a shared folder
func (s *CollaborationStore) RemoveMember(folderID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder, ok := s.sharedFolders[folderID]
	if !ok {
		return os.ErrNotExist
	}

	// Cannot remove owner
	if folder.OwnerID == userID {
		return os.ErrInvalid
	}

	var newMembers []FolderMember
	for _, m := range folder.Members {
		if m.UserID != userID {
			newMembers = append(newMembers, m)
		}
	}
	folder.Members = newMembers

	return s.save()
}

// UpdateMemberPermission updates a member's permission level
func (s *CollaborationStore) UpdateMemberPermission(folderID, userID string, permission SharePermission) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder, ok := s.sharedFolders[folderID]
	if !ok {
		return os.ErrNotExist
	}

	for i, m := range folder.Members {
		if m.UserID == userID {
			folder.Members[i].Permission = permission
			return s.save()
		}
	}

	return os.ErrNotExist
}

// DeleteSharedFolder deletes a shared folder
func (s *CollaborationStore) DeleteSharedFolder(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sharedFolders[id]; !ok {
		return os.ErrNotExist
	}

	delete(s.sharedFolders, id)
	return s.save()
}

// ==================== Invitations ====================

// CreateInvite creates a new share invitation
func (s *CollaborationStore) CreateInvite(folderID, inviterID, inviterName, inviteeID, inviteeEmail string, permission SharePermission, message string, expiresIn time.Duration) (*ShareInvite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder, ok := s.sharedFolders[folderID]
	if !ok {
		return nil, os.ErrNotExist
	}

	invite := &ShareInvite{
		ID:             uuid.New().String(),
		SharedFolderID: folderID,
		FolderName:     folder.Name,
		InviterID:      inviterID,
		InviterName:    inviterName,
		InviteeID:      inviteeID,
		InviteeEmail:   inviteeEmail,
		Permission:     permission,
		Status:         InvitePending,
		Message:        message,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(expiresIn),
	}

	s.invites[invite.ID] = invite

	if err := s.save(); err != nil {
		delete(s.invites, invite.ID)
		return nil, err
	}

	return invite, nil
}

// GetInvite retrieves an invitation by ID
func (s *CollaborationStore) GetInvite(id string) (*ShareInvite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	invite, ok := s.invites[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return invite, nil
}

// ListPendingInvites returns pending invitations for a user
func (s *CollaborationStore) ListPendingInvites(userID string) []*ShareInvite {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ShareInvite
	now := time.Now()
	for _, invite := range s.invites {
		if invite.InviteeID == userID && invite.Status == InvitePending && invite.ExpiresAt.After(now) {
			result = append(result, invite)
		}
	}
	return result
}

// AcceptInvite accepts an invitation
func (s *CollaborationStore) AcceptInvite(inviteID, userID, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	invite, ok := s.invites[inviteID]
	if !ok {
		return os.ErrNotExist
	}

	if invite.InviteeID != userID {
		return os.ErrPermission
	}

	if invite.Status != InvitePending {
		return os.ErrInvalid
	}

	if time.Now().After(invite.ExpiresAt) {
		invite.Status = InviteExpired
		s.save()
		return os.ErrInvalid
	}

	// Add user to folder
	folder, ok := s.sharedFolders[invite.SharedFolderID]
	if !ok {
		return os.ErrNotExist
	}

	folder.Members = append(folder.Members, FolderMember{
		UserID:     userID,
		Username:   username,
		Permission: invite.Permission,
		AddedAt:    time.Now(),
		AddedBy:    invite.InviterID,
	})

	now := time.Now()
	invite.Status = InviteAccepted
	invite.RespondedAt = &now

	return s.save()
}

// DeclineInvite declines an invitation
func (s *CollaborationStore) DeclineInvite(inviteID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	invite, ok := s.invites[inviteID]
	if !ok {
		return os.ErrNotExist
	}

	if invite.InviteeID != userID {
		return os.ErrPermission
	}

	now := time.Now()
	invite.Status = InviteDeclined
	invite.RespondedAt = &now

	return s.save()
}

// HasPermission checks if a user has the specified permission on a folder
func (s *CollaborationStore) HasPermission(folderID, userID string, required SharePermission) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folder, ok := s.sharedFolders[folderID]
	if !ok {
		return false
	}

	for _, m := range folder.Members {
		if m.UserID == userID {
			switch required {
			case PermissionRead:
				return true // All permissions include read
			case PermissionWrite:
				return m.Permission == PermissionWrite || m.Permission == PermissionAdmin
			case PermissionAdmin:
				return m.Permission == PermissionAdmin
			}
		}
	}
	return false
}

// load reads data from disk
func (s *CollaborationStore) load() error {
	// Load shared folders
	foldersPath := filepath.Join(s.dataDir, "collaboration", "folders.json")
	if data, err := os.ReadFile(foldersPath); err == nil {
		var folders []*SharedFolder
		if err := json.Unmarshal(data, &folders); err != nil {
			return err
		}
		for _, f := range folders {
			s.sharedFolders[f.ID] = f
		}
	}

	// Load invites
	invitesPath := filepath.Join(s.dataDir, "collaboration", "invites.json")
	if data, err := os.ReadFile(invitesPath); err == nil {
		var invites []*ShareInvite
		if err := json.Unmarshal(data, &invites); err != nil {
			return err
		}
		for _, i := range invites {
			s.invites[i.ID] = i
		}
	}

	return nil
}

// save writes data to disk
func (s *CollaborationStore) save() error {
	// Save shared folders
	folders := make([]*SharedFolder, 0, len(s.sharedFolders))
	for _, f := range s.sharedFolders {
		folders = append(folders, f)
	}
	foldersData, err := json.MarshalIndent(folders, "", "  ")
	if err != nil {
		return err
	}
	foldersPath := filepath.Join(s.dataDir, "collaboration", "folders.json")
	if err := os.WriteFile(foldersPath, foldersData, 0640); err != nil {
		return err
	}

	// Save invites
	invites := make([]*ShareInvite, 0, len(s.invites))
	for _, i := range s.invites {
		invites = append(invites, i)
	}
	invitesData, err := json.MarshalIndent(invites, "", "  ")
	if err != nil {
		return err
	}
	invitesPath := filepath.Join(s.dataDir, "collaboration", "invites.json")
	return os.WriteFile(invitesPath, invitesData, 0640)
}

