package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	vesselRepo  = "Yash121l/Vessel"
	vesselBin   = "/usr/local/bin/vessel"
	vesselSvc   = "vessel"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Vessel to the latest release from GitHub",
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Vessel %s — checking for updates…\n", Version)

	latest, downloadURL, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("could not fetch release info: %w", err)
	}

	// Strip leading 'v' for comparison
	latestVer := strings.TrimPrefix(latest, "v")
	currentVer := strings.TrimPrefix(Version, "v")

	if latestVer == currentVer {
		fmt.Printf("Already on the latest version (%s). Nothing to do.\n", Version)
		return nil
	}

	fmt.Printf("New version available: %s → %s\n", currentVer, latestVer)
	fmt.Printf("Downloading from %s…\n", downloadURL)

	if err := downloadAndReplace(downloadURL, vesselBin); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("Binary updated to %s\n", latestVer)

	// Restart the systemd service if running under systemd
	if isSystemdManaged() {
		fmt.Println("Restarting vessel service…")
		out, err := exec.Command("systemctl", "restart", vesselSvc).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl restart failed: %s: %w", strings.TrimSpace(string(out)), err)
		}
		fmt.Println("Service restarted. Vessel is now running the new version.")
	} else {
		fmt.Println("Not running under systemd — please restart Vessel manually.")
	}

	return nil
}

// fetchLatestRelease queries the GitHub Releases API and returns the tag name
// and the download URL for the correct platform binary.
func fetchLatestRelease() (tag, downloadURL string, err error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", vesselRepo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	assetName := platformAssetName()
	for _, a := range release.Assets {
		if a.Name == assetName {
			return release.TagName, a.BrowserDownloadURL, nil
		}
	}
	return "", "", fmt.Errorf("no asset named %q in release %s", assetName, release.TagName)
}

// platformAssetName returns the expected binary name for the current OS/arch.
func platformAssetName() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return "vessel_linux_amd64"
	case "arm64":
		return "vessel_linux_arm64"
	case "arm":
		return "vessel_linux_armv7"
	default:
		return fmt.Sprintf("vessel_linux_%s", arch)
	}
}

// downloadAndReplace downloads the binary at url, writes it next to the
// current binary as a temp file, then atomically replaces the current binary.
func downloadAndReplace(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	// Write to a temp file in the same directory so rename is atomic
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".vessel-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName) // clean up on failure; no-op if rename succeeded
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return fmt.Errorf("write binary: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	tmp.Close()

	// Atomic replace
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// isSystemdManaged returns true when the vessel service unit exists.
func isSystemdManaged() bool {
	paths := []string{
		"/etc/systemd/system/vessel.service",
		"/lib/systemd/system/vessel.service",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}
