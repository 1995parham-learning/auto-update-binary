package update

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Manifest represents the server-side version manifest
type Manifest struct {
	SchemaVersion int                  `json:"schema_version"`
	Generated     time.Time            `json:"generated"`
	Components    map[string]Component `json:"components"`
}

// Component represents a single updatable binary
type Component struct {
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	ReleaseDate time.Time        `json:"release_date"`
	Changelog   string           `json:"changelog,omitempty"`
	Assets      map[string]Asset `json:"assets"`
}

// Asset represents a downloadable binary for a specific platform
type Asset struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// CurrentPlatform returns the platform key for the current OS/arch
func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// Version represents a semantic version
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a semantic version string
func ParseVersion(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format: %s", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// String returns the version as a string
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare compares two versions. Returns -1 if v < other, 0 if equal, 1 if v > other
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// LessThan returns true if v is less than other
func (v Version) LessThan(other Version) bool {
	return v.Compare(other) < 0
}
