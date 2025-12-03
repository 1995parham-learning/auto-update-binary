package update

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Checker handles version checking against the update server
type Checker struct {
	serverURL  string
	httpClient *http.Client
	logger     *slog.Logger
}

// CheckResult contains the result of a version check
type CheckResult struct {
	Component       string
	CurrentVersion  Version
	LatestVersion   Version
	UpdateAvailable bool
	Asset           *Asset
}

// NewChecker creates a new version checker
func NewChecker(serverURL string, logger *slog.Logger) *Checker {
	return &Checker{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GetManifest fetches the current version manifest from the server
func (c *Checker) GetManifest(ctx context.Context) (*Manifest, error) {
	url := c.serverURL + "/v1/manifest.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "nametag-updater/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}

	return &manifest, nil
}

// Check checks if an update is available for a component
func (c *Checker) Check(ctx context.Context, component string, currentVersion Version) (*CheckResult, error) {
	c.logger.Info("checking for updates",
		"component", component,
		"current_version", currentVersion.String(),
	)

	manifest, err := c.GetManifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}

	comp, ok := manifest.Components[component]
	if !ok {
		return nil, fmt.Errorf("component %q not found in manifest", component)
	}

	latestVersion, err := ParseVersion(comp.Version)
	if err != nil {
		return nil, fmt.Errorf("parse latest version: %w", err)
	}

	result := &CheckResult{
		Component:       component,
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: currentVersion.LessThan(latestVersion),
	}

	if result.UpdateAvailable {
		platform := CurrentPlatform()
		asset, ok := comp.Assets[platform]
		if !ok {
			return nil, fmt.Errorf("no asset found for platform %q", platform)
		}
		result.Asset = &asset

		c.logger.Info("update available",
			"component", component,
			"current", currentVersion.String(),
			"latest", latestVersion.String(),
		)
	} else {
		c.logger.Info("no update available",
			"component", component,
			"current", currentVersion.String(),
		)
	}

	return result, nil
}
