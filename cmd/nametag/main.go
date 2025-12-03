package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"

	"github.com/nametag/nametag/internal/ipc"
	"github.com/nametag/nametag/internal/platform"
	"github.com/nametag/nametag/internal/update"
)

var (
	version   = "1.0.0"
	commit    = "none"
	date      = "unknown"
	serverURL = "http://localhost:8080"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Clean up any old binaries from previous updates
	_ = platform.CleanupOldBinaries()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	os.Args = os.Args[1:] // Shift args for subcommand flags
	flag.CommandLine = flag.NewFlagSet(cmd, flag.ExitOnError)

	switch cmd {
	case "version":
		cmdVersion()
	case "check":
		cmdCheck(logger)
	case "update":
		cmdUpdate(logger)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("nametag - A self-updating application demo")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  nametag <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  version   Show version information")
	fmt.Println("  check     Check for updates")
	fmt.Println("  update    Download and apply updates")
	fmt.Println("  help      Show this help message")
}

func cmdVersion() {
	fmt.Printf("nametag version %s\n", version)
	fmt.Printf("  commit:   %s\n", commit)
	fmt.Printf("  built:    %s\n", date)
	fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func cmdCheck(logger *slog.Logger) {
	server := flag.String("server", serverURL, "Update server URL")
	flag.Parse()

	currentVersion, err := update.ParseVersion(version)
	if err != nil {
		logger.Error("failed to parse current version", "error", err)
		os.Exit(1)
	}

	checker := update.NewChecker(*server, logger)
	ctx := context.Background()

	result, err := checker.Check(ctx, "nametag", currentVersion)
	if err != nil {
		logger.Error("failed to check for updates", "error", err)
		os.Exit(1)
	}

	if result.UpdateAvailable {
		fmt.Printf("Update available!\n")
		fmt.Printf("  Current: %s\n", result.CurrentVersion.String())
		fmt.Printf("  Latest:  %s\n", result.LatestVersion.String())
		fmt.Printf("\nRun 'nametag update' to install the update.\n")
	} else {
		fmt.Printf("You are running the latest version (%s)\n", version)
	}
}

func cmdUpdate(logger *slog.Logger) {
	server := flag.String("server", serverURL, "Update server URL")
	flag.Parse()

	currentVersion, err := update.ParseVersion(version)
	if err != nil {
		logger.Error("failed to parse current version", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Step 1: Check for updates
	logger.Info("checking for updates")
	checker := update.NewChecker(*server, logger)

	result, err := checker.Check(ctx, "nametag", currentVersion)
	if err != nil {
		logger.Error("failed to check for updates", "error", err)
		os.Exit(1)
	}

	if !result.UpdateAvailable {
		fmt.Printf("You are running the latest version (%s)\n", version)
		return
	}

	fmt.Printf("Downloading update %s -> %s\n", result.CurrentVersion.String(), result.LatestVersion.String())

	// Step 2: Download the new binary
	downloader := update.NewDownloader(logger)
	tempPath := platform.TempDownloadPath(result.LatestVersion.String())

	// Build full download URL
	downloadURL := *server + result.Asset.URL

	downloadResult, err := downloader.Download(ctx, downloadURL, tempPath, func(downloaded, total int64) {
		if total > 0 {
			pct := float64(downloaded) / float64(total) * 100
			fmt.Printf("\rDownloading: %.1f%%", pct)
		}
	})
	if err != nil {
		logger.Error("download failed", "error", err)
		os.Remove(tempPath)
		os.Exit(1)
	}
	fmt.Println() // Newline after progress

	// Step 3: Verify checksum
	logger.Info("verifying checksum")
	if downloadResult.SHA256 != result.Asset.SHA256 {
		logger.Error("checksum mismatch",
			"expected", result.Asset.SHA256,
			"got", downloadResult.SHA256,
		)
		os.Remove(tempPath)
		os.Exit(1)
	}

	// Step 4: Prepare update command
	execPath, err := platform.GetExecutablePath()
	if err != nil {
		logger.Error("failed to get executable path", "error", err)
		os.Remove(tempPath)
		os.Exit(1)
	}

	updaterPath, err := platform.GetUpdaterPath()
	if err != nil {
		logger.Error("failed to get updater path", "error", err)
		os.Remove(tempPath)
		os.Exit(1)
	}

	// Check if updater exists
	if _, err := os.Stat(updaterPath); err != nil {
		logger.Error("updater not found", "path", updaterPath)
		os.Remove(tempPath)
		os.Exit(1)
	}

	cmd := &ipc.UpdateCommand{
		Action:         ipc.ActionUpdate,
		TargetBinary:   execPath,
		NewBinaryPath:  tempPath,
		BackupPath:     platform.GetBackupPath(execPath),
		ExpectedSHA256: result.Asset.SHA256,
		RestartBinary:  execPath,
		RestartArgs:    []string{"version"},
		ParentPID:      os.Getpid(),
	}

	// Step 5: Write command file
	cmdFile := platform.TempCommandPath()
	if err := cmd.WriteToFile(cmdFile); err != nil {
		logger.Error("failed to write command file", "error", err)
		os.Remove(tempPath)
		os.Exit(1)
	}

	// Step 6: Spawn updater
	fmt.Println("Launching updater...")
	proc := exec.Command(updaterPath, "--command-file", cmdFile)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	platform.ConfigureDetached(proc)

	if err := proc.Start(); err != nil {
		logger.Error("failed to start updater", "error", err)
		os.Remove(tempPath)
		os.Remove(cmdFile)
		os.Exit(1)
	}

	logger.Info("updater started, exiting for update", "updater_pid", proc.Process.Pid)
	fmt.Println("Update in progress, please wait...")

	// Step 7: Exit to allow updater to replace us
	os.Exit(0)
}
