# Nametag - Self-Updating Application

version := "1.0.0"
commit := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

ldflags := "-s -w -X main.version=" + version + " -X main.commit=" + commit + " -X main.date=" + date

platforms := "darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64"

# Show available commands
default:
    @just --list

# Build binaries for current platform
build:
    @echo "Building nametag and nametag-up..."
    go build -ldflags "{{ldflags}}" -o bin/nametag ./cmd/nametag
    go build -ldflags "{{ldflags}}" -o bin/nametag-up ./cmd/nametag-up
    go build -ldflags "{{ldflags}}" -o bin/server ./cmd/server
    @echo "Done! Binaries in ./bin/"

# Build for a specific platform (e.g., just build-platform linux-amd64)
build-platform platform:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building for {{platform}}..."
    GOOS=$(echo {{platform}} | cut -d- -f1)
    GOARCH=$(echo {{platform}} | cut -d- -f2)
    EXT=""
    if [[ "$GOOS" == "windows" ]]; then
        EXT=".exe"
    fi
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "{{ldflags}}" -o bin/nametag-{{platform}}$EXT ./cmd/nametag
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "{{ldflags}}" -o bin/nametag-up-{{platform}}$EXT ./cmd/nametag-up

# Build for all platforms
build-all:
    #!/usr/bin/env bash
    set -euo pipefail
    for platform in {{platforms}}; do
        just build-platform $platform
    done

# Run tests
test:
    go test -v -race ./...

# Create release structure for server
release: build-all
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Creating release structure..."
    mkdir -p releases/nametag/{{version}}
    mkdir -p releases/nametag-up/{{version}}
    for platform in {{platforms}}; do
        ext=""
        if echo "$platform" | grep -q "windows"; then
            ext=".exe"
        fi
        cp bin/nametag-${platform}${ext} releases/nametag/{{version}}/ 2>/dev/null || true
        cp bin/nametag-up-${platform}${ext} releases/nametag-up/{{version}}/ 2>/dev/null || true
    done
    echo "Release files in ./releases/"

# Clean build artifacts
clean:
    rm -rf bin/
    rm -rf releases/

# Start the update server
server: build
    ./bin/server -assets ./releases

# Check for updates (development helper)
check: build
    ./bin/nametag check -server http://localhost:8080

# Show version info
version:
    @echo "Version: {{version}}"
    @echo "Commit:  {{commit}}"
    @echo "Date:    {{date}}"

# Initialize go module dependencies
init:
    go mod tidy

# Format code
fmt:
    go fmt ./...

# Run linter
lint:
    go vet ./...
