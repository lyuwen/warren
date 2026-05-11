package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lfu/warren/internal/events"
)

// ArtifactProfile tracks files and repositories touched by an agent session
type ArtifactProfile struct {
	AgentID string `json:"agent_id"`

	// RepoRoots contains detected repository root directories
	RepoRoots []string `json:"repo_roots"`

	// FilesVisited tracks all files that were read
	FilesVisited []string `json:"files_visited"`

	// FilesEdited tracks all files that were modified
	FilesEdited []string `json:"files_edited"`

	// LastUpdated is when this profile was last modified
	LastUpdated time.Time `json:"last_updated"`

	// Statistics
	TotalReads  int `json:"total_reads"`
	TotalEdits  int `json:"total_edits"`
	TotalWrites int `json:"total_writes"`
}

// ArtifactProfileManager manages artifact profiles for all agent sessions
type ArtifactProfileManager struct {
	profiles map[string]*ArtifactProfile
	mu       sync.RWMutex
}

// NewArtifactProfileManager creates a new artifact profile manager
func NewArtifactProfileManager() *ArtifactProfileManager {
	return &ArtifactProfileManager{
		profiles: make(map[string]*ArtifactProfile),
	}
}

// GetProfile retrieves the artifact profile for an agent
func (m *ArtifactProfileManager) GetProfile(agentID string) (*ArtifactProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, exists := m.profiles[agentID]
	if !exists {
		return nil, fmt.Errorf("no artifact profile found for agent %s", agentID)
	}

	return profile, nil
}

// GetOrCreateProfile retrieves or creates an artifact profile for an agent
func (m *ArtifactProfileManager) GetOrCreateProfile(agentID string) *ArtifactProfile {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.profiles[agentID]
	if !exists {
		profile = &ArtifactProfile{
			AgentID:      agentID,
			RepoRoots:    []string{},
			FilesVisited: []string{},
			FilesEdited:  []string{},
			LastUpdated:  time.Now(),
		}
		m.profiles[agentID] = profile
	}

	return profile
}

// ProcessActivity updates the artifact profile based on an activity event
func (m *ArtifactProfileManager) ProcessActivity(activity *events.AgentActivityEvent) error {
	if activity.ActivityType != "file" {
		return nil // Only process file activities
	}

	filePath, ok := activity.Metadata["file_path"]
	if !ok || filePath == "" {
		return nil // No file path in metadata
	}

	operation, ok := activity.Metadata["operation"]
	if !ok {
		operation = "unknown"
	}

	profile := m.GetOrCreateProfile(activity.AgentID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update statistics
	switch operation {
	case "read":
		profile.TotalReads++
		if !contains(profile.FilesVisited, filePath) {
			profile.FilesVisited = append(profile.FilesVisited, filePath)
		}
	case "edit":
		profile.TotalEdits++
		if !contains(profile.FilesEdited, filePath) {
			profile.FilesEdited = append(profile.FilesEdited, filePath)
		}
		// Edited files are also visited
		if !contains(profile.FilesVisited, filePath) {
			profile.FilesVisited = append(profile.FilesVisited, filePath)
		}
	case "write":
		profile.TotalWrites++
		if !contains(profile.FilesEdited, filePath) {
			profile.FilesEdited = append(profile.FilesEdited, filePath)
		}
		if !contains(profile.FilesVisited, filePath) {
			profile.FilesVisited = append(profile.FilesVisited, filePath)
		}
	}

	// Detect repository root
	if repoRoot := findRepoRoot(filePath); repoRoot != "" {
		if !contains(profile.RepoRoots, repoRoot) {
			profile.RepoRoots = append(profile.RepoRoots, repoRoot)
		}
	}

	profile.LastUpdated = time.Now()

	return nil
}

// ProcessActivities processes multiple activity events
func (m *ArtifactProfileManager) ProcessActivities(activities []*events.AgentActivityEvent) error {
	for _, activity := range activities {
		if err := m.ProcessActivity(activity); err != nil {
			return err
		}
	}
	return nil
}

// ListProfiles returns all artifact profiles
func (m *ArtifactProfileManager) ListProfiles() []*ArtifactProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]*ArtifactProfile, 0, len(m.profiles))
	for _, profile := range m.profiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// RemoveProfile removes an artifact profile
func (m *ArtifactProfileManager) RemoveProfile(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.profiles[agentID]; !exists {
		return fmt.Errorf("no artifact profile found for agent %s", agentID)
	}

	delete(m.profiles, agentID)
	return nil
}

// Count returns the number of profiles
func (m *ArtifactProfileManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.profiles)
}

// findRepoRoot searches for a .git directory in the file's path hierarchy
func findRepoRoot(filePath string) string {
	// Clean and make absolute
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return ""
	}

	// Start from the file's directory
	dir := filepath.Dir(absPath)

	// Walk up the directory tree
	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .git
			break
		}
		dir = parent
	}

	return ""
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// GetFilesByRepo returns files grouped by repository
func (p *ArtifactProfile) GetFilesByRepo() map[string][]string {
	result := make(map[string][]string)

	for _, filePath := range p.FilesVisited {
		repoRoot := findRepoRoot(filePath)
		if repoRoot == "" {
			repoRoot = "unknown"
		}
		result[repoRoot] = append(result[repoRoot], filePath)
	}

	return result
}

// GetEditedFilesByRepo returns edited files grouped by repository
func (p *ArtifactProfile) GetEditedFilesByRepo() map[string][]string {
	result := make(map[string][]string)

	for _, filePath := range p.FilesEdited {
		repoRoot := findRepoRoot(filePath)
		if repoRoot == "" {
			repoRoot = "unknown"
		}
		result[repoRoot] = append(result[repoRoot], filePath)
	}

	return result
}

// GetRelativePaths returns file paths relative to their repository roots
func (p *ArtifactProfile) GetRelativePaths() []string {
	result := []string{}

	for _, filePath := range p.FilesVisited {
		repoRoot := findRepoRoot(filePath)
		if repoRoot != "" {
			relPath := strings.TrimPrefix(filePath, repoRoot)
			relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
			result = append(result, relPath)
		} else {
			result = append(result, filePath)
		}
	}

	return result
}
