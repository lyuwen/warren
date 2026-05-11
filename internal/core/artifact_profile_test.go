package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lfu/warren/internal/events"
)

func TestArtifactProfileManager_GetOrCreateProfile(t *testing.T) {
	manager := NewArtifactProfileManager()

	agentID := "agent-1"
	profile := manager.GetOrCreateProfile(agentID)

	if profile.AgentID != agentID {
		t.Errorf("expected agent ID %s, got %s", agentID, profile.AgentID)
	}

	if len(profile.FilesVisited) != 0 {
		t.Errorf("expected empty files visited, got %d", len(profile.FilesVisited))
	}

	// Get the same profile again
	profile2 := manager.GetOrCreateProfile(agentID)
	if profile != profile2 {
		t.Error("expected same profile instance")
	}
}

func TestArtifactProfileManager_GetProfile(t *testing.T) {
	manager := NewArtifactProfileManager()

	// Try to get non-existent profile
	_, err := manager.GetProfile("non-existent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}

	// Create and get profile
	agentID := "agent-1"
	manager.GetOrCreateProfile(agentID)

	profile, err := manager.GetProfile(agentID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if profile.AgentID != agentID {
		t.Errorf("expected agent ID %s, got %s", agentID, profile.AgentID)
	}
}

func TestArtifactProfileManager_ProcessActivity_Read(t *testing.T) {
	manager := NewArtifactProfileManager()

	activity := &events.AgentActivityEvent{
		AgentID:      "agent-1",
		ActivityType: "file",
		Content:      "Read tool: file_path=/home/user/project/main.go",
		Metadata: map[string]string{
			"operation": "read",
			"file_path": "/home/user/project/main.go",
		},
		Timestamp: time.Now(),
	}

	err := manager.ProcessActivity(activity)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	profile, _ := manager.GetProfile("agent-1")

	if profile.TotalReads != 1 {
		t.Errorf("expected 1 read, got %d", profile.TotalReads)
	}

	if len(profile.FilesVisited) != 1 {
		t.Errorf("expected 1 file visited, got %d", len(profile.FilesVisited))
	}

	if profile.FilesVisited[0] != "/home/user/project/main.go" {
		t.Errorf("expected /home/user/project/main.go, got %s", profile.FilesVisited[0])
	}
}

func TestArtifactProfileManager_ProcessActivity_Edit(t *testing.T) {
	manager := NewArtifactProfileManager()

	activity := &events.AgentActivityEvent{
		AgentID:      "agent-1",
		ActivityType: "file",
		Content:      "Edit tool: file_path=/home/user/project/main.go",
		Metadata: map[string]string{
			"operation": "edit",
			"file_path": "/home/user/project/main.go",
		},
		Timestamp: time.Now(),
	}

	err := manager.ProcessActivity(activity)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	profile, _ := manager.GetProfile("agent-1")

	if profile.TotalEdits != 1 {
		t.Errorf("expected 1 edit, got %d", profile.TotalEdits)
	}

	if len(profile.FilesEdited) != 1 {
		t.Errorf("expected 1 file edited, got %d", len(profile.FilesEdited))
	}

	if len(profile.FilesVisited) != 1 {
		t.Errorf("expected 1 file visited (edited files are also visited), got %d", len(profile.FilesVisited))
	}
}

func TestArtifactProfileManager_ProcessActivity_Write(t *testing.T) {
	manager := NewArtifactProfileManager()

	activity := &events.AgentActivityEvent{
		AgentID:      "agent-1",
		ActivityType: "file",
		Content:      "Write tool: file_path=/home/user/project/new.go",
		Metadata: map[string]string{
			"operation": "write",
			"file_path": "/home/user/project/new.go",
		},
		Timestamp: time.Now(),
	}

	err := manager.ProcessActivity(activity)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	profile, _ := manager.GetProfile("agent-1")

	if profile.TotalWrites != 1 {
		t.Errorf("expected 1 write, got %d", profile.TotalWrites)
	}

	if len(profile.FilesEdited) != 1 {
		t.Errorf("expected 1 file edited (writes count as edits), got %d", len(profile.FilesEdited))
	}

	if len(profile.FilesVisited) != 1 {
		t.Errorf("expected 1 file visited, got %d", len(profile.FilesVisited))
	}
}

func TestArtifactProfileManager_ProcessActivity_NonFile(t *testing.T) {
	manager := NewArtifactProfileManager()

	activity := &events.AgentActivityEvent{
		AgentID:      "agent-1",
		ActivityType: "chat",
		Content:      "user: hello",
		Metadata:     map[string]string{},
		Timestamp:    time.Now(),
	}

	err := manager.ProcessActivity(activity)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should not create a profile for non-file activities
	_, err = manager.GetProfile("agent-1")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestArtifactProfileManager_ProcessActivity_DuplicateFiles(t *testing.T) {
	manager := NewArtifactProfileManager()

	filePath := "/home/user/project/main.go"

	// Read the same file twice
	for i := 0; i < 2; i++ {
		activity := &events.AgentActivityEvent{
			AgentID:      "agent-1",
			ActivityType: "file",
			Content:      "Read tool",
			Metadata: map[string]string{
				"operation": "read",
				"file_path": filePath,
			},
			Timestamp: time.Now(),
		}
		manager.ProcessActivity(activity)
	}

	profile, _ := manager.GetProfile("agent-1")

	if profile.TotalReads != 2 {
		t.Errorf("expected 2 reads, got %d", profile.TotalReads)
	}

	if len(profile.FilesVisited) != 1 {
		t.Errorf("expected 1 unique file visited, got %d", len(profile.FilesVisited))
	}
}

func TestArtifactProfileManager_ProcessActivities(t *testing.T) {
	manager := NewArtifactProfileManager()

	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "agent-1",
			ActivityType: "file",
			Metadata: map[string]string{
				"operation": "read",
				"file_path": "/home/user/project/file1.go",
			},
			Timestamp: time.Now(),
		},
		{
			AgentID:      "agent-1",
			ActivityType: "file",
			Metadata: map[string]string{
				"operation": "edit",
				"file_path": "/home/user/project/file2.go",
			},
			Timestamp: time.Now(),
		},
		{
			AgentID:      "agent-1",
			ActivityType: "chat",
			Content:      "user: hello",
			Timestamp:    time.Now(),
		},
	}

	err := manager.ProcessActivities(activities)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	profile, _ := manager.GetProfile("agent-1")

	if profile.TotalReads != 1 {
		t.Errorf("expected 1 read, got %d", profile.TotalReads)
	}

	if profile.TotalEdits != 1 {
		t.Errorf("expected 1 edit, got %d", profile.TotalEdits)
	}

	if len(profile.FilesVisited) != 2 {
		t.Errorf("expected 2 files visited, got %d", len(profile.FilesVisited))
	}
}

func TestArtifactProfileManager_ListProfiles(t *testing.T) {
	manager := NewArtifactProfileManager()

	manager.GetOrCreateProfile("agent-1")
	manager.GetOrCreateProfile("agent-2")
	manager.GetOrCreateProfile("agent-3")

	profiles := manager.ListProfiles()

	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
}

func TestArtifactProfileManager_RemoveProfile(t *testing.T) {
	manager := NewArtifactProfileManager()

	manager.GetOrCreateProfile("agent-1")

	err := manager.RemoveProfile("agent-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = manager.GetProfile("agent-1")
	if err == nil {
		t.Error("expected error for removed profile")
	}

	// Try to remove non-existent profile
	err = manager.RemoveProfile("non-existent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestArtifactProfileManager_Count(t *testing.T) {
	manager := NewArtifactProfileManager()

	if manager.Count() != 0 {
		t.Errorf("expected 0 profiles, got %d", manager.Count())
	}

	manager.GetOrCreateProfile("agent-1")
	manager.GetOrCreateProfile("agent-2")

	if manager.Count() != 2 {
		t.Errorf("expected 2 profiles, got %d", manager.Count())
	}
}

func TestFindRepoRoot(t *testing.T) {
	// Create a temporary directory structure with a .git directory
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	gitDir := filepath.Join(repoDir, ".git")
	subDir := filepath.Join(repoDir, "src", "pkg")

	os.MkdirAll(gitDir, 0755)
	os.MkdirAll(subDir, 0755)

	testFile := filepath.Join(subDir, "main.go")

	// Test finding repo root from nested file
	repoRoot := findRepoRoot(testFile)
	if repoRoot != repoDir {
		t.Errorf("expected repo root %s, got %s", repoDir, repoRoot)
	}

	// Test file outside repo
	outsideFile := filepath.Join(tmpDir, "outside.txt")
	repoRoot = findRepoRoot(outsideFile)
	if repoRoot != "" {
		t.Errorf("expected empty repo root for file outside repo, got %s", repoRoot)
	}
}

func TestArtifactProfile_GetFilesByRepo(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	repo1 := filepath.Join(tmpDir, "repo1")
	repo2 := filepath.Join(tmpDir, "repo2")

	os.MkdirAll(filepath.Join(repo1, ".git"), 0755)
	os.MkdirAll(filepath.Join(repo2, ".git"), 0755)

	profile := &ArtifactProfile{
		AgentID: "agent-1",
		FilesVisited: []string{
			filepath.Join(repo1, "file1.go"),
			filepath.Join(repo1, "file2.go"),
			filepath.Join(repo2, "file3.go"),
		},
	}

	filesByRepo := profile.GetFilesByRepo()

	if len(filesByRepo) != 2 {
		t.Errorf("expected 2 repos, got %d", len(filesByRepo))
	}

	if len(filesByRepo[repo1]) != 2 {
		t.Errorf("expected 2 files in repo1, got %d", len(filesByRepo[repo1]))
	}

	if len(filesByRepo[repo2]) != 1 {
		t.Errorf("expected 1 file in repo2, got %d", len(filesByRepo[repo2]))
	}
}

func TestArtifactProfile_GetEditedFilesByRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repo1 := filepath.Join(tmpDir, "repo1")

	os.MkdirAll(filepath.Join(repo1, ".git"), 0755)

	profile := &ArtifactProfile{
		AgentID: "agent-1",
		FilesEdited: []string{
			filepath.Join(repo1, "file1.go"),
			filepath.Join(repo1, "file2.go"),
		},
	}

	editedByRepo := profile.GetEditedFilesByRepo()

	if len(editedByRepo) != 1 {
		t.Errorf("expected 1 repo, got %d", len(editedByRepo))
	}

	if len(editedByRepo[repo1]) != 2 {
		t.Errorf("expected 2 edited files in repo1, got %d", len(editedByRepo[repo1]))
	}
}

func TestArtifactProfile_GetRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	gitDir := filepath.Join(repoDir, ".git")

	os.MkdirAll(gitDir, 0755)

	profile := &ArtifactProfile{
		AgentID: "agent-1",
		FilesVisited: []string{
			filepath.Join(repoDir, "main.go"),
			filepath.Join(repoDir, "src", "pkg", "util.go"),
		},
	}

	relativePaths := profile.GetRelativePaths()

	if len(relativePaths) != 2 {
		t.Errorf("expected 2 relative paths, got %d", len(relativePaths))
	}

	if relativePaths[0] != "main.go" {
		t.Errorf("expected main.go, got %s", relativePaths[0])
	}

	expectedPath := filepath.Join("src", "pkg", "util.go")
	if relativePaths[1] != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, relativePaths[1])
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !contains(slice, "b") {
		t.Error("expected to find 'b' in slice")
	}

	if contains(slice, "d") {
		t.Error("expected not to find 'd' in slice")
	}

	if contains([]string{}, "a") {
		t.Error("expected not to find 'a' in empty slice")
	}
}
