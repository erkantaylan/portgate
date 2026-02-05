package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const releaseURL = "https://api.github.com/repos/erkantaylan/portgate/releases/latest"

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// isNewer returns true if remote is a higher semver than local.
// Both should be in "vMAJOR.MINOR.PATCH" format.
func isNewer(local, remote string) bool {
	parse := func(v string) (int, int, int, bool) {
		v = strings.TrimPrefix(v, "v")
		parts := strings.SplitN(v, ".", 3)
		if len(parts) != 3 {
			return 0, 0, 0, false
		}
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		patch, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, 0, 0, false
		}
		return major, minor, patch, true
	}

	lMaj, lMin, lPat, lok := parse(local)
	rMaj, rMin, rPat, rok := parse(remote)
	if !lok || !rok {
		return false
	}

	if rMaj != lMaj {
		return rMaj > lMaj
	}
	if rMin != lMin {
		return rMin > lMin
	}
	return rPat > lPat
}

// binaryAssetName returns the expected GitHub release asset name for this platform.
func binaryAssetName() string {
	if runtime.GOOS == "windows" {
		return "portgate-windows-amd64.exe"
	}
	return "portgate-linux-amd64"
}

// checkLatestRelease fetches the latest GitHub release info.
func checkLatestRelease() (*githubRelease, error) {
	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}
	return &rel, nil
}

// downloadURL returns the download URL for our platform from the release.
func (r *githubRelease) downloadURL() string {
	name := binaryAssetName()
	for _, a := range r.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func cmdVersion() {
	fmt.Printf("portgate %s\n", version)
}

func cmdUpdate() {
	fmt.Printf("Current version: %s\n", version)
	fmt.Println("Checking for updates...")

	rel, err := checkLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !isNewer(version, rel.TagName) {
		fmt.Printf("Already up to date (%s)\n", version)
		return
	}

	dlURL := rel.downloadURL()
	if dlURL == "" {
		fmt.Fprintf(os.Stderr, "No binary found for %s/%s in release %s\n",
			runtime.GOOS, runtime.GOARCH, rel.TagName)
		os.Exit(1)
	}

	fmt.Printf("Downloading %s...\n", rel.TagName)

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot determine executable path: %v\n", err)
		os.Exit(1)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot resolve executable path: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.Get(dlURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Download failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, "portgate-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create temp file: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		os.Exit(1)
	}
	tmp.Close()

	if err := selfReplace(exe, tmpPath); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated to %s\n", rel.TagName)
}

// backgroundUpdateCheck logs if a newer version is available (non-blocking).
func backgroundUpdateCheck() {
	if version == "dev" {
		return
	}
	rel, err := checkLatestRelease()
	if err != nil {
		return
	}
	if isNewer(version, rel.TagName) {
		log.Printf("Update available: %s → %s (run 'portgate update' to upgrade)", version, rel.TagName)
	}
}
