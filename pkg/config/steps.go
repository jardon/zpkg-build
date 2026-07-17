package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Step string

const (
	StepPull    Step = "pull"
	StepBuild   Step = "build"
	StepPackage Step = "package"
	StepExport  Step = "export"
)

var StepOrder = []Step{StepPull, StepBuild, StepPackage, StepExport}

type StepState struct {
	Step       Step      `json:"step"`
	Status     string    `json:"status"`
	SourceHash string    `json:"source_hash"`
	ConfigHash string    `json:"config_hash"`
	Timestamp  time.Time `json:"timestamp"`
}

type Tracker struct {
	stateDir string
}

func NewTracker(workspacePath string) *Tracker {
	return &Tracker{
		stateDir: filepath.Join(workspacePath, ".zpkg-build-state"),
	}
}

func (t *Tracker) IsStepCached(step Step, currentSrcHash, currentConfigHash string) bool {
	stateFile := filepath.Join(t.stateDir, string(step)+".json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return false
	}

	var state StepState
	if err := json.Unmarshal(data, &state); err != nil {
		return false
	}

	return state.Status == "completed" &&
		state.SourceHash == currentSrcHash &&
		state.ConfigHash == currentConfigHash
}

func (t *Tracker) MarkStepComplete(step Step, currentSrcHash, currentConfigHash string) error {
	if err := os.MkdirAll(t.stateDir, 0755); err != nil {
		return err
	}

	state := StepState{
		Step:       step,
		Status:     "completed",
		SourceHash: currentSrcHash,
		ConfigHash: currentConfigHash,
		Timestamp:  time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	stateFile := filepath.Join(t.stateDir, string(step)+".json")
	return os.WriteFile(stateFile, data, 0644)
}

func (t *Tracker) InvalidateFrom(step Step) error {
	invalidate := false
	for _, s := range StepOrder {
		if s == step {
			invalidate = true
		}
		if invalidate {
			stateFile := filepath.Join(t.stateDir, string(s)+".json")
			_ = os.Remove(stateFile)
		}
	}
	return nil
}

func (t *Tracker) GetState(step Step) (*StepState, error) {
	stateFile := filepath.Join(t.stateDir, string(step)+".json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("no state found for step %s: %w", step, err)
	}

	var state StepState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}
