# Nametag

A self-updating application written in Go.

## Architecture

Nametag uses a **two-binary architecture** for reliable self-updates:

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

**Update Flow:**
1. `nametag` checks the server for a new version
2. Downloads new binary to a temp location
3. Verifies SHA256 checksum
4. Spawns `nametag-up` with update instructions
5. `nametag` exits
6. `nametag-up` waits for parent process to exit
7. Replaces the old binary with the new one (atomic)
8. Launches the updated `nametag`

## Building

Requires Go and [just](https://github.com/casey/just).

```bash
# Build for current platform
just build

# Build for all platforms (darwin, linux, windows)
just build-all

# Run tests
just test

# Create release structure for server
just version=1.0.0 release
```

## Usage

### Main Application

```bash
# Show version
./bin/nametag version

# Check for updates
./bin/nametag check -server http://localhost:8080

# Download and apply updates
./bin/nametag update -server http://localhost:8080
```

### Update Server

```bash
# Start the server (serves from ./releases directory)
./bin/server -addr :8080 -assets ./releases

# Or use just
just server
```

The server expects binaries organized as:

```text
releases/
├── nametag/
│   └── 1.0.0/
│       ├── nametag-darwin-amd64
│       ├── nametag-darwin-arm64
│       ├── nametag-linux-amd64
│       ├── nametag-linux-arm64
│       └── nametag-windows-amd64.exe
└── nametag-up/
    └── 1.0.0/
        ├── nametag-up-darwin-amd64
        └── ...
```

### API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /v1/manifest.json` | Version manifest with checksums |
| `GET /v1/download/{component}/{platform}/{version}` | Download binary |
| `GET /health` | Health check |

### Example Manifest

```json
{
  "schema_version": 1,
  "generated": "2025-01-15T10:30:00Z",
  "components": {
    "nametag": {
      "name": "nametag",
      "version": "1.1.0",
      "release_date": "2025-01-15T10:00:00Z",
      "assets": {
        "linux-amd64": {
          "url": "/v1/download/nametag/linux-amd64/1.1.0",
          "size": 5242880,
          "sha256": "abc123..."
        }
      }
    }
  }
}
```

## Platform Support

| Platform | Status |
|----------|--------|
| Linux (amd64, arm64) | Supported |
| macOS (amd64, arm64) | Supported |
| Windows (amd64) | Supported |

### Platform-Specific Notes

**Windows:**
- Running executables can be renamed but not overwritten
- Old binaries are cleaned up on next startup

**macOS:**
- Quarantine extended attribute is automatically removed from downloaded binaries

## Testing the Update Flow

1. Build version 1.0.0:
   ```bash
   just version=1.0.0 release
   ```

2. Copy binaries to a test location:
   ```bash
   cp bin/nametag /tmp/test/nametag
   cp bin/nametag-up /tmp/test/nametag-up
   ```

3. Build version 1.1.0:
   ```bash
   just version=1.1.0 release
   ```

4. Start the server:
   ```bash
   just server
   ```

5. Run update from the test location:
   ```bash
   cd /tmp/test
   ./nametag update -server http://localhost:8080
   ```

6. Verify the update:
   ```bash
   ./nametag version
   ```

## Design Decisions

1. **Two-Binary Architecture**: Separates update logic from application logic. The updater is minimal and stable, while the main app can change frequently.
2. **SHA256 Verification**: Each download is verified against the checksum in the manifest.
3. **Atomic Replacement**: Uses rename operations for atomic updates. On failure, the old binary is restored.
4. **IPC via File**: Update commands are passed between processes via a JSON file, which is more reliable than command-line arguments for complex data.
5. **Structured Logging**: Uses Go's `log/slog` package for consistent, queryable logs.

## Project Structure

```text
nametag/
├── cmd/
│   ├── nametag/        # Main application
│   ├── nametag-up/     # Updater binary
│   └── server/         # Update server
├── internal/
│   ├── update/         # Update logic (checker, downloader, replacer)
│   ├── platform/       # Platform-specific code
│   └── ipc/            # Inter-process communication
├── go.mod
├── justfile
└── README.md
```
