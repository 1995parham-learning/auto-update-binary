package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nametag/nametag/internal/update"
)

var (
	version = "dev"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	addr := flag.String("addr", ":8080", "Server address")
	assetsDir := flag.String("assets", "./releases", "Directory containing release binaries")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("nametag-server version %s\n", version)
		return
	}

	server := &Server{
		assetsDir: *assetsDir,
		logger:    logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/manifest.json", server.handleManifest)
	mux.HandleFunc("/v1/download/", server.handleDownload)
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/", server.handleRoot)

	logger.Info("starting update server",
		"addr", *addr,
		"assets_dir", *assetsDir,
	)

	if err := http.ListenAndServe(*addr, mux); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

type Server struct {
	assetsDir string
	logger    *slog.Logger
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Nametag Update Server\n")
	fmt.Fprintf(w, "\nEndpoints:\n")
	fmt.Fprintf(w, "  GET /v1/manifest.json - Version manifest\n")
	fmt.Fprintf(w, "  GET /v1/download/{component}/{platform}/{version} - Download binary\n")
	fmt.Fprintf(w, "  GET /health - Health check\n")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("manifest requested", "remote", r.RemoteAddr)

	manifest, err := s.generateManifest()
	if err != nil {
		s.logger.Error("failed to generate manifest", "error", err)
		http.Error(w, "Failed to generate manifest", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(manifest)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Parse path: /v1/download/{component}/{platform}/{version}
	path := strings.TrimPrefix(r.URL.Path, "/v1/download/")
	parts := strings.Split(path, "/")

	if len(parts) != 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	component := parts[0]
	platform := parts[1]
	version := parts[2]

	s.logger.Info("download requested",
		"component", component,
		"platform", platform,
		"version", version,
		"remote", r.RemoteAddr,
	)

	// Validate inputs
	if !isValidComponent(component) || !isValidPlatform(platform) {
		http.Error(w, "Invalid component or platform", http.StatusBadRequest)
		return
	}

	// Construct file path
	filename := fmt.Sprintf("%s-%s-%s", component, platform, version)
	if strings.HasPrefix(platform, "windows") {
		filename += ".exe"
	}

	filePath := filepath.Join(s.assetsDir, component, version, filename)

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Warn("file not found", "path", filePath)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Serve file
	http.ServeFile(w, r, filePath)
}

func (s *Server) generateManifest() (*update.Manifest, error) {
	manifest := &update.Manifest{
		SchemaVersion: 1,
		Generated:     time.Now().UTC(),
		Components:    make(map[string]update.Component),
	}

	// Scan assets directory for components
	components := []string{"nametag", "nametag-up"}
	platforms := []string{
		"darwin-amd64", "darwin-arm64",
		"linux-amd64", "linux-arm64",
		"windows-amd64",
	}

	for _, comp := range components {
		compDir := filepath.Join(s.assetsDir, comp)
		if _, err := os.Stat(compDir); os.IsNotExist(err) {
			continue
		}

		// Find latest version
		versions, err := os.ReadDir(compDir)
		if err != nil {
			continue
		}

		var latestVersion string
		for _, v := range versions {
			if v.IsDir() {
				latestVersion = v.Name()
			}
		}

		if latestVersion == "" {
			continue
		}

		component := update.Component{
			Name:        comp,
			Version:     latestVersion,
			ReleaseDate: time.Now().UTC(),
			Assets:      make(map[string]update.Asset),
		}

		// Find assets for each platform
		for _, plat := range platforms {
			filename := fmt.Sprintf("%s-%s-%s", comp, plat, latestVersion)
			if strings.HasPrefix(plat, "windows") {
				filename += ".exe"
			}

			filePath := filepath.Join(compDir, latestVersion, filename)
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			// Compute SHA256
			hash, err := computeSHA256(filePath)
			if err != nil {
				s.logger.Warn("failed to compute hash", "file", filePath, "error", err)
				continue
			}

			component.Assets[plat] = update.Asset{
				URL:    fmt.Sprintf("/v1/download/%s/%s/%s", comp, plat, latestVersion),
				Size:   info.Size(),
				SHA256: hash,
			}
		}

		if len(component.Assets) > 0 {
			manifest.Components[comp] = component
		}
	}

	return manifest, nil
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func isValidComponent(c string) bool {
	return c == "nametag" || c == "nametag-up"
}

func isValidPlatform(p string) bool {
	valid := map[string]bool{
		"darwin-amd64":  true,
		"darwin-arm64":  true,
		"linux-amd64":   true,
		"linux-arm64":   true,
		"windows-amd64": true,
	}
	return valid[p]
}
