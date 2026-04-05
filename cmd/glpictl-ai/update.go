package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const repo = "giulianotesta7/glpictl-ai"

// runUpdate handles the "update" subcommand.
// Returns exit code: 0 on success, 1 on error.
func runUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	fs.Parse(args)

	fmt.Println("Checking for updates...")

	// Get current version
	currentVersion := version
	if currentVersion == "dev" {
		fmt.Println("Cannot update development version. Please install from a release.")
		return 1
	}

	// Fetch latest release
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to check for updates: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: GitHub API returned status %d\n", resp.StatusCode)
		return 1
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read response: %v\n", err)
		return 1
	}

	latestVersion := extractTagName(string(body))
	if latestVersion == "" {
		fmt.Println("Error: could not determine latest version")
		return 1
	}

	// Compare versions
	if latestVersion == currentVersion {
		fmt.Printf("You are running the latest version: %s\n", currentVersion)
		return 0
	}

	fmt.Printf("New version available: %s (current: %s)\n", latestVersion, currentVersion)

	// Detect OS and arch
	targetOS := runtime.GOOS
	arch := runtime.GOARCH
	switch arch {
	case "x86_64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}

	// Build download URL
	binaryName := "glpictl-ai"
	if targetOS == "windows" {
		binaryName = "glpictl-ai.exe"
	}
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s-%s-%s-%s",
		repo, latestVersion, binaryName, latestVersion, targetOS, arch)

	// Get installation directory
	installDir := getInstallDir()
	if err := os.MkdirAll(installDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create installation directory: %v\n", err)
		return 1
	}

	destPath := filepath.Join(installDir, binaryName)

	fmt.Printf("Downloading %s...\n", downloadURL)

	// Download binary
	downloadResp, err := http.Get(downloadURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to download: %v\n", err)
		return 1
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		// Try fallback URL (without version in name)
		fallbackURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s-%s-%s",
			repo, latestVersion, binaryName, targetOS, arch)
		fmt.Printf("Primary URL failed, trying fallback...\n")

		downloadResp, err = http.Get(fallbackURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to download: %v\n", err)
			return 1
		}
		defer downloadResp.Body.Close()

		if downloadResp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Error: failed to download (status %d)\n", downloadResp.StatusCode)
			return 1
		}
		downloadURL = fallbackURL
	}

	// Save to temp file
	tempFile, err := os.CreateTemp("", "glpictl-ai-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create temp file: %v\n", err)
		return 1
	}
	tempPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempPath)

	// Copy response to temp file
	if _, err := io.Copy(tempFile, downloadResp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save download: %v\n", err)
		return 1
	}
	tempFile.Close()

	// Make executable
	if err := os.Chmod(tempPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to make executable: %v\n", err)
		return 1
	}

	// Replace old binary
	if err := os.Rename(tempPath, destPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to install: %v\n", err)
		return 1
	}

	fmt.Printf("Updated to %s!\n", latestVersion)
	fmt.Printf("Installed to: %s\n", destPath)

	return 0
}

// getInstallDir returns the default installation directory.
func getInstallDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), ".local", "bin")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

// extractTagName extracts the tag_name from a GitHub API response.
func extractTagName(body string) string {
	prefix := `"tag_name":"`
	idx := strings.Index(body, prefix)
	if idx == -1 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(body[start:], `"`)
	if end == -1 {
		return ""
	}
	return body[start : start+end]
}
