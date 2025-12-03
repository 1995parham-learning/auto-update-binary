package ipc

import (
	"encoding/json"
	"fmt"
	"os"
)

// Action represents the type of update action
type Action string

const (
	ActionUpdate   Action = "update"
	ActionRollback Action = "rollback"
)

// UpdateCommand is passed from main app to updater
type UpdateCommand struct {
	Action         Action   `json:"action"`
	TargetBinary   string   `json:"target_binary"`
	NewBinaryPath  string   `json:"new_binary_path"`
	BackupPath     string   `json:"backup_path"`
	ExpectedSHA256 string   `json:"expected_sha256"`
	RestartBinary  string   `json:"restart_binary"`
	RestartArgs    []string `json:"restart_args"`
	ParentPID      int      `json:"parent_pid"`
}

// WriteToFile writes the command to a JSON file
func (c *UpdateCommand) WriteToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ReadFromFile reads the command from a JSON file
func ReadFromFile(path string) (*UpdateCommand, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var cmd UpdateCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, fmt.Errorf("unmarshal command: %w", err)
	}

	return &cmd, nil
}

// Cleanup removes the command file
func Cleanup(path string) {
	_ = os.Remove(path)
}
