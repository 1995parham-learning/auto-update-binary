//go:build !windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ConfigureDetached configures a command to run as a fully detached process
func ConfigureDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}
}

// WaitForProcessExit waits for a process to exit with timeout
func WaitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		process, err := os.FindProcess(pid)
		if err != nil {
			return nil // Process gone
		}

		// On Unix, FindProcess always succeeds, so we try to signal
		err = process.Signal(syscall.Signal(0))
		if err != nil {
			return nil // Process gone
		}

		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for process %d", pid)
}

// AtomicReplace performs Unix atomic binary replacement
func AtomicReplace(target, newFile, backup string) error {
	// Remove any existing backup
	_ = os.Remove(backup)

	// Backup old file
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("backup old: %w", err)
	}

	// Move new file to target
	if err := os.Rename(newFile, target); err != nil {
		_ = os.Rename(backup, target) // Rollback
		return fmt.Errorf("install new: %w", err)
	}

	// Set permissions
	if err := os.Chmod(target, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	return nil
}

// ScheduleCleanup removes old binary immediately on Unix
func ScheduleCleanup(path string) {
	_ = os.Remove(path)
}

// RemoveQuarantine removes the quarantine extended attribute on macOS
// This is a no-op on Linux
func RemoveQuarantine(path string) error {
	// Only relevant on macOS - attempt xattr removal
	cmd := exec.Command("xattr", "-d", "com.apple.quarantine", path)
	_ = cmd.Run() // Ignore errors - file might not have quarantine attribute
	return nil
}

// BinaryExtension returns the extension for executable binaries
func BinaryExtension() string {
	return ""
}
