package update

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/1995parham-learning/auto-update-binary/internal/platform"
)

// Replacer handles atomic binary replacement
type Replacer struct {
	logger *slog.Logger
}

// NewReplacer creates a new replacer
func NewReplacer(logger *slog.Logger) *Replacer {
	return &Replacer{
		logger: logger,
	}
}

// Replace performs atomic binary replacement
func (r *Replacer) Replace(targetPath, newBinaryPath, backupPath string) error {
	r.logger.Info("replacing binary",
		"target", targetPath,
		"new", newBinaryPath,
		"backup", backupPath,
	)

	// Verify new binary exists
	if _, err := os.Stat(newBinaryPath); err != nil {
		return fmt.Errorf("new binary not found: %w", err)
	}

	// Perform platform-specific atomic replacement
	if err := platform.AtomicReplace(targetPath, newBinaryPath, backupPath); err != nil {
		return fmt.Errorf("atomic replace: %w", err)
	}

	// Remove quarantine on macOS
	if err := platform.RemoveQuarantine(targetPath); err != nil {
		r.logger.Warn("failed to remove quarantine", "error", err)
	}

	r.logger.Info("binary replaced successfully")
	return nil
}

// Rollback restores the backup binary
func (r *Replacer) Rollback(targetPath, backupPath string) error {
	r.logger.Warn("rolling back update",
		"target", targetPath,
		"backup", backupPath,
	)

	// Check if backup exists
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	// Remove failed new binary
	_ = os.Remove(targetPath)

	// Restore backup
	if err := os.Rename(backupPath, targetPath); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	r.logger.Info("rollback complete")
	return nil
}

// ValidateAfterUpdate performs post-update validation
func (r *Replacer) ValidateAfterUpdate(binaryPath string) error {
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("stat binary: %w", err)
	}

	// Check binary is executable (on Unix)
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable")
	}

	return nil
}
