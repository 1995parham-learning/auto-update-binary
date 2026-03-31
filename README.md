# Nametag

A self-updating application written in Go that demonstrates a reliable two-binary update architecture
with SHA256 verification, atomic replacement, and automatic rollback on failure.

> [!note]
> This project was built as a take-home assignment from [Nametag](https://getnametag.com/).

## Architecture

### Two-Binary Design

The update process is split between two binaries to avoid the problem of a process replacing itself while running:

- **`nametag`** — the main application. Checks for updates, downloads the new binary, then hands off to the updater.
- **`nametag-up`** — the updater. Waits for the main app to exit, swaps the binary atomically, and relaunches.

```text
┌─────────────────┐      HTTPS       ┌─────────────────┐
│  Update Server  │◄─────────────────│    nametag      │
│  (serves bins)  │                  │  (main app)     │
└─────────────────┘                  └────────┬────────┘
        ▲                                     │ spawn
        │            HTTPS                    ▼
        └────────────────────────────┌────────────────┐
                                     │   nametag-up   │
                                     │   (updater)    │
                                     └────────────────┘
```

### Update Flow (step by step)

1. `nametag` fetches `/v1/manifest.json` from the update server
2. Compares the manifest version against its embedded version using semver
3. Downloads the new binary to a temp file (`/tmp/nametag-update-<version>`)
4. Computes SHA256 of the download and verifies it against the manifest checksum
5. Writes an `UpdateCommand` JSON file to `/tmp/nametag-update-cmd.json` containing:
   - paths (target binary, new binary, backup)
   - expected SHA256
   - restart instructions
   - parent PID
6. Spawns `nametag-up --command-file /tmp/nametag-update-cmd.json` as a detached process
7. `nametag` exits
8. `nametag-up` reads the command file, waits up to 30s for the parent PID to exit
9. Re-verifies the SHA256 checksum of the new binary
10. Performs atomic replacement: rename old binary to `.old`, rename new binary into place
11. Validates the new binary is executable
12. Launches the updated `nametag` (with `version` subcommand to confirm success)
13. Cleans up the backup and command file

If step 10 fails, `nametag-up` automatically rolls back by restoring the `.old` backup.

### IPC via File

The main app and updater communicate through a JSON command file (`ipc.UpdateCommand`) rather than
CLI arguments. This keeps the interface clean and supports complex data (paths, checksums, restart args)
without shell escaping issues.

### Update Server

The server is a simple HTTP server that:

- Scans a `releases/` directory on disk to auto-generate the manifest
- Computes SHA256 checksums on the fly for each asset
- Picks the latest version per component by lexicographic directory name ordering
- Serves binary downloads directly from the filesystem

### Platform-Specific Behavior

| Concern              | Unix (Linux/macOS)                                        | Windows                                                    |
| -------------------- | --------------------------------------------------------- | ---------------------------------------------------------- |
| Detached process     | `Setsid: true` (new session)                              | `CREATE_NEW_PROCESS_GROUP \| DETACHED_PROCESS`             |
| Wait for parent exit | Signal(0) polling, 100ms interval                         | `WaitForSingleObject` with timeout                         |
| Atomic replace       | `os.Rename` old to `.old`, then new to target             | Same rename strategy (Windows allows renaming running exe) |
| Cleanup              | Immediate `os.Remove` of `.old` backup                    | Deferred to next startup (running exe can't be deleted)    |
| Quarantine           | `xattr -d com.apple.quarantine` on macOS (no-op on Linux) | No-op                                                      |
| Binary extension     | (none)                                                    | `.exe`                                                     |

## Prerequisites

- Go 1.25+
- [just](https://github.com/casey/just) command runner

## Building

```bash
# Build nametag, nametag-up, and server for current platform
just build

# Build for a specific platform
just build-platform linux-amd64

# Build for all platforms (darwin-amd64, darwin-arm64, linux-amd64, linux-arm64, windows-amd64)
just build-all

# Create release directory structure (builds all platforms first)
just version=1.1.0 release
```

Version, commit hash, and build date are injected via `-ldflags` at build time.

## Running

### Start the Update Server

```bash
# Build and start the server (serves from ./releases directory)
just server

# Or run manually with options
./bin/server -addr :8080 -assets ./releases
```

### Use the Main Application

```bash
# Show version, commit, build date, and platform
./bin/nametag version

# Check if an update is available
./bin/nametag check -server http://localhost:8080

# Download and apply the update
./bin/nametag update -server http://localhost:8080
```

### Server API

| Endpoint                                            | Description                                                        |
| --------------------------------------------------- | ------------------------------------------------------------------ |
| `GET /health`                                       | Returns `{"status":"ok"}`                                          |
| `GET /v1/manifest.json`                             | Auto-generated manifest with versions, sizes, and SHA256 checksums |
| `GET /v1/download/{component}/{platform}/{version}` | Serves the binary file                                             |

The server expects release binaries organized as:

```text
releases/
├── nametag/
│   └── 1.1.0/
│       ├── nametag-darwin-amd64
│       ├── nametag-darwin-arm64
│       ├── nametag-linux-amd64
│       ├── nametag-linux-arm64
│       └── nametag-windows-amd64.exe
└── nametag-up/
    └── 1.1.0/
        ├── nametag-up-darwin-amd64
        ├── nametag-up-darwin-arm64
        ├── nametag-up-linux-amd64
        ├── nametag-up-linux-arm64
        └── nametag-up-windows-amd64.exe
```

File naming convention: `{component}-{os}-{arch}` (version is encoded in the directory path, not the filename).

## Testing the Update Flow

End-to-end test of a v1.0.0 to v1.1.0 update:

```bash
# 1. Build v1.0.0 release binaries
just version=1.0.0 release

# 2. Copy v1.0.0 binaries to a test location
mkdir -p /tmp/nametag-test
cp bin/nametag-linux-amd64 /tmp/nametag-test/nametag
cp bin/nametag-up-linux-amd64 /tmp/nametag-test/nametag-up
chmod +x /tmp/nametag-test/*

# 3. Verify it reports v1.0.0
/tmp/nametag-test/nametag version

# 4. Build v1.1.0 release binaries (overwrites the releases/ directory)
just version=1.1.0 release

# 5. Start the update server (serves v1.1.0 from ./releases)
just server &

# 6. Check for updates from the test location
/tmp/nametag-test/nametag check -server http://localhost:8080

# 7. Apply the update
/tmp/nametag-test/nametag update -server http://localhost:8080

# 8. Verify the update succeeded
/tmp/nametag-test/nametag version
# Should now report v1.1.0
```

### What to look for

- Step 6 should print "Update available! Current: 1.0.0, Latest: 1.1.0"
- Step 7 should show download progress, launch the updater, and exit
- Step 8 should confirm the version changed to 1.1.0
- No `.old` backup files should remain in `/tmp/nametag-test/` (cleaned up automatically on Unix)

## Project Structure

```text
├── cmd/
│   ├── nametag/          # Main application (version, check, update commands)
│   ├── nametag-up/       # Updater binary (reads command file, replaces binary)
│   └── server/           # HTTP update server (manifest generation, file serving)
├── internal/
│   ├── ipc/              # UpdateCommand struct and JSON serialization
│   ├── platform/         # OS-specific code (process mgmt, atomic replace, paths)
│   │   ├── exec_unix.go
│   │   ├── exec_windows.go
│   │   └── paths.go
│   └── update/           # Core update logic
│       ├── checker.go    # Version checking against server manifest
│       ├── downloader.go # HTTP download with progress and SHA256
│       ├── manifest.go   # Manifest types and semver parsing
│       └── replacer.go   # Atomic binary replacement with rollback
├── go.mod
├── justfile
└── README.md
```

## Design Decisions

1. **Two-binary architecture** — a running process cannot reliably replace itself. The updater is intentionally minimal and stable so it rarely needs updating itself.
2. **SHA256 verification** — the checksum is verified twice: once after download (by `nametag`) and once before replacement (by `nametag-up`), guarding against both network corruption and TOCTOU races.
3. **Atomic replacement via rename** — `os.Rename` is atomic on both Unix and Windows (NTFS). On failure, the `.old` backup is restored automatically.
4. **IPC via JSON file** — more reliable than CLI arguments for passing complex structured data between processes. The command file is cleaned up after use.
5. **Structured logging** — `nametag` uses `slog.TextHandler`, `nametag-up` uses `slog.JSONHandler` (distinct format makes it easy to tell which binary is logging).
6. **Platform abstraction** — all OS-specific behavior (process management, file operations, binary extensions) is isolated in `internal/platform/` behind build tags.
