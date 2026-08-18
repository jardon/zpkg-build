package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTracker_MarkAndRead(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(dir)

	if err := tracker.MarkStepComplete(StepPull, "src123", "cfg456"); err != nil {
		t.Fatalf("MarkStepComplete() error: %v", err)
	}

	state, err := tracker.GetState(StepPull)
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}

	if state.Step != StepPull {
		t.Errorf("step = %q, want %q", state.Step, StepPull)
	}
	if state.Status != "completed" {
		t.Errorf("status = %q, want %q", state.Status, "completed")
	}
	if state.SourceHash != "src123" {
		t.Errorf("source_hash = %q, want %q", state.SourceHash, "src123")
	}
	if state.ConfigHash != "cfg456" {
		t.Errorf("config_hash = %q, want %q", state.ConfigHash, "cfg456")
	}
}

func TestTracker_IsStepCached(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(dir)

	t.Run("not cached when never marked", func(t *testing.T) {
		if tracker.IsStepCached(StepPull, "src", "cfg") {
			t.Error("step should not be cached")
		}
	})

	t.Run("cached when hashes match", func(t *testing.T) {
		tracker.MarkStepComplete(StepPull, "src", "cfg")
		if !tracker.IsStepCached(StepPull, "src", "cfg") {
			t.Error("step should be cached with matching hashes")
		}
	})

	t.Run("not cached when source hash differs", func(t *testing.T) {
		if tracker.IsStepCached(StepPull, "different_src", "cfg") {
			t.Error("step should not be cached with different source hash")
		}
	})

	t.Run("not cached when config hash differs", func(t *testing.T) {
		if tracker.IsStepCached(StepPull, "src", "different_cfg") {
			t.Error("step should not be cached with different config hash")
		}
	})

	t.Run("other step not cached", func(t *testing.T) {
		if tracker.IsStepCached(StepBuild, "src", "cfg") {
			t.Error("other step should not be cached")
		}
	})
}

func TestTracker_InvalidateFrom(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(dir)

	// Mark all steps
	for _, step := range StepOrder {
		tracker.MarkStepComplete(step, "src", "cfg")
	}

	// Invalidate from build (should remove build, package, export)
	tracker.InvalidateFrom(StepBuild)

	for _, step := range StepOrder {
		stateFile := filepath.Join(tracker.stateDir, string(step)+".json")
		exists := fileExists(stateFile)

		switch step {
		case StepPull:
			if !exists {
				t.Error("pull step should still exist")
			}
		case StepBuild, StepPackage, StepExport:
			if exists {
				t.Errorf("step %s should have been invalidated", step)
			}
		}
	}
}

func TestTracker_InvalidateFromFirst(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(dir)

	for _, step := range StepOrder {
		tracker.MarkStepComplete(step, "src", "cfg")
	}

	// Invalidate from pull (should remove all)
	tracker.InvalidateFrom(StepPull)

	for _, step := range StepOrder {
		stateFile := filepath.Join(tracker.stateDir, string(step)+".json")
		if fileExists(stateFile) {
			t.Errorf("step %s should have been invalidated", step)
		}
	}
}

func TestTracker_GetState_NotFound(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(dir)

	_, err := tracker.GetState(StepPull)
	if err == nil {
		t.Error("expected error for nonexistent state file")
	}
}

func TestNewTracker(t *testing.T) {
	tracker := NewTracker("/some/workspace")
	expected := filepath.Join("/some/workspace", ".zpkg-build-state")
	if tracker.stateDir != expected {
		t.Errorf("stateDir = %q, want %q", tracker.stateDir, expected)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
