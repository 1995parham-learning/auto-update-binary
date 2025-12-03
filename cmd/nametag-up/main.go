package main

import (
	"flag"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/1995parham-learning/auto-update-binary/internal/ipc"
	"github.com/1995parham-learning/auto-update-binary/internal/platform"
	"github.com/1995parham-learning/auto-update-binary/internal/update"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cmdFile := flag.String("command-file", "", "Path to command JSON file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		logger.Info("nametag-up",
			"version", version,
			"commit", commit,
			"date", date,
		)
		os.Exit(0)
	}

	if *cmdFile == "" {
		logger.Error("command-file is required")
		os.Exit(1)
	}

	cmd, err := ipc.ReadFromFile(*cmdFile)
	if err != nil {
		logger.Error("failed to read command file", "error", err)
		os.Exit(1)
	}

	// Clean up command file when done
	defer ipc.Cleanup(*cmdFile)

	if err := executeUpdate(logger, cmd); err != nil {
		logger.Error("update failed", "error", err)

		// Attempt rollback on failure
		if cmd.Action == ipc.ActionUpdate {
			replacer := update.NewReplacer(logger)
			if rollbackErr := replacer.Rollback(cmd.TargetBinary, cmd.BackupPath); rollbackErr != nil {
				logger.Error("rollback also failed", "error", rollbackErr)
			}
		}
		os.Exit(1)
	}

	logger.Info("update completed successfully")
}

func executeUpdate(logger *slog.Logger, cmd *ipc.UpdateCommand) error {
	logger.Info("executing update",
		"action", cmd.Action,
		"target", cmd.TargetBinary,
		"parent_pid", cmd.ParentPID,
	)

	// Step 1: Wait for parent process to exit
	logger.Info("waiting for parent process to exit", "pid", cmd.ParentPID)
	if err := platform.WaitForProcessExit(cmd.ParentPID, 30*time.Second); err != nil {
		return err
	}
	logger.Info("parent process has exited")

	// Step 2: Verify the new binary checksum
	logger.Info("verifying new binary checksum")
	if err := update.VerifyChecksum(cmd.NewBinaryPath, cmd.ExpectedSHA256); err != nil {
		return err
	}
	logger.Info("checksum verified")

	// Step 3: Perform atomic replacement
	replacer := update.NewReplacer(logger)
	if err := replacer.Replace(cmd.TargetBinary, cmd.NewBinaryPath, cmd.BackupPath); err != nil {
		return err
	}

	// Step 4: Validate the new binary
	if err := replacer.ValidateAfterUpdate(cmd.TargetBinary); err != nil {
		return err
	}

	// Step 5: Start the new binary
	if cmd.RestartBinary != "" {
		logger.Info("starting new binary", "path", cmd.RestartBinary)

		proc := exec.Command(cmd.RestartBinary, cmd.RestartArgs...)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		platform.ConfigureDetached(proc)

		if err := proc.Start(); err != nil {
			return err
		}

		logger.Info("new binary started", "pid", proc.Process.Pid)
	}

	// Step 6: Schedule cleanup of old binary
	platform.ScheduleCleanup(cmd.BackupPath)

	return nil
}
