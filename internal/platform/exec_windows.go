//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// ConfigureDetached configures a command to run as a fully detached process
func ConfigureDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
}

// WaitForProcessExit waits for a process to exit with timeout
func WaitForProcessExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// Process might already be gone
		return nil
	}
	defer windows.CloseHandle(handle)

	event, err := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("wait for process: %w", err)
	}
	if event == windows.WAIT_TIMEOUT {
		return fmt.Errorf("timeout waiting for process %d", pid)
	}
	return nil
}

// AtomicReplace performs Windows-safe binary replacement
// On Windows, we rename the old file rather than delete it
func AtomicReplace(target, newFile, backup string) error {
	// Step 1: Remove any existing backup
	_ = os.Remove(backup)

	// Step 2: Rename running executable to backup
	// This works even while the exe is running!
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("rename old: %w", err)
	}

	// Step 3: Move new file to target path
	if err := os.Rename(newFile, target); err != nil {
		// Rollback: restore old file
		_ = os.Rename(backup, target)
		return fmt.Errorf("rename new: %w", err)
	}

	// Step 4: Hide the backup file
	hideFile(backup)

	return nil
}

func hideFile(path string) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_ = syscall.SetFileAttributes(ptr, syscall.FILE_ATTRIBUTE_HIDDEN)
}

// ScheduleCleanup marks file for deletion on next startup
// We can't delete the old exe while it might still be referenced
func ScheduleCleanup(path string) {
	// On Windows, we leave the .old file and clean it up on next startup
	// The cleanup is handled by the main app at startup
}

// RemoveQuarantine is a no-op on Windows
func RemoveQuarantine(path string) error {
	return nil
}

// BinaryExtension returns the extension for executable binaries
func BinaryExtension() string {
	return ".exe"
}
