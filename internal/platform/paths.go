package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// GetExecutablePath returns the path to the current executable
func GetExecutablePath() (string, error) {
	return os.Executable()
}

// GetUpdaterPath returns the path to the updater binary
func GetUpdaterPath() (string, error) {
	execPath, err := GetExecutablePath()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(execPath)
	updaterName := "nametag-up" + BinaryExtension()

	return filepath.Join(dir, updaterName), nil
}

// GetBackupPath returns the backup path for a binary
func GetBackupPath(binaryPath string) string {
	return binaryPath + ".old"
}

// CleanupOldBinaries removes any leftover .old backup files
func CleanupOldBinaries() error {
	execPath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(execPath)
	base := filepath.Base(execPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".old") && strings.HasPrefix(name, strings.TrimSuffix(base, filepath.Ext(base))) {
			oldPath := filepath.Join(dir, name)
			_ = os.Remove(oldPath) // Best effort cleanup
		}
	}

	// Also clean up temp files from interrupted updates
	tmpPattern := filepath.Join(os.TempDir(), "nametag-update-*")
	matches, _ := filepath.Glob(tmpPattern)
	for _, match := range matches {
		_ = os.Remove(match)
	}

	return nil
}

// TempDownloadPath returns a temporary path for downloading an update
func TempDownloadPath(version string) string {
	return filepath.Join(os.TempDir(), "nametag-update-"+version+BinaryExtension())
}

// TempCommandPath returns a temporary path for the update command file
func TempCommandPath() string {
	return filepath.Join(os.TempDir(), "nametag-update-cmd.json")
}
