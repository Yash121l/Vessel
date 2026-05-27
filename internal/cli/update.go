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
	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/logger"
)

const (
	vesselRepo  = "Yash121l/Vessel"
	vesselBin   = "/usr/local/bin/vessel"
	vesselSvc   = "vessel"
)

var noRestart bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Vessel to the latest release from GitHub",
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&noRestart, "no-restart", false, "Download and replace binary only; skip service restart")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err == nil {
		logger.Init(debug, cfg.DataDir)
		defer logger.Close()
	}

	fmt.Printf("Vessel %s — checking for updates…\n", Version)
	logger.Infof("Vessel %s — checking for updates…", Version)

	latest, downloadURL, err := fetchLatestRelease()
	if err != nil {
		logger.Errorf("could not fetch release info: %v", err)
		return fmt.Errorf("could not fetch release info: %w", err)
	}


	// Strip leading 'v' for comparison
	latestVer := strings.TrimPrefix(latest, "v")
	currentVer := strings.TrimPrefix(Version, "v")

	if latestVer == currentVer {
		fmt.Printf("Already on the latest version (%s). Nothing to do.\n", Version)
		logger.Infof("Already on the latest version (%s). Nothing to do.", Version)
		return nil
	}

	fmt.Printf("New version available: %s → %s\n", currentVer, latestVer)
	fmt.Printf("Downloading from %s…\n", downloadURL)
	logger.Infof("New version available: %s → %s. Downloading from %s", currentVer, latestVer, downloadURL)

	if err := downloadAndReplace(downloadURL, vesselBin); err != nil {
		logger.Errorf("download failed: %v", err)
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("Binary updated to %s\n", latestVer)
	logger.Infof("Binary successfully updated to %s", latestVer)

	// Restart the systemd service unless --no-restart was passed
	// (the server's selfUpdate handler handles restart itself after flushing SSE).
	if noRestart {
		fmt.Println("Skipping service restart (--no-restart).")
		logger.Infof("Skipping service restart (--no-restart).")
		return nil
	}

	// Restart the systemd service if running under systemd
	if isSystemdManaged() {
		fmt.Println("Restarting vessel service…")
		logger.Infof("Restarting vessel service via systemctl...")
		out, err := exec.Command("systemctl", "restart", vesselSvc).CombinedOutput()
		if err != nil {
			logger.Errorf("systemctl restart failed: %s: %v", strings.TrimSpace(string(out)), err)
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

// downloadAndReplace downloads the binary at url, writes it to a temp file,
// then replaces the destination binary. The temp file is written to os.TempDir()
// to avoid permission issues with /usr/local/bin, then copied into place.
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

	// Write to a temp file in os.TempDir() — always writable, avoids
	// permission errors when dest is in /usr/local/bin.
	tmp, err := os.CreateTemp("", ".vessel-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName) // clean up on failure; no-op if copy succeeded
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return fmt.Errorf("write binary: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	tmp.Close()

	// Copy temp file over the destination (works across filesystems/devices).
	if err := copyFile(tmpName, dest); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Write to a temp file next to dst so the final rename is atomic on the
	// same filesystem. If that also fails due to permissions, write directly.
	dir := filepath.Dir(dst)
	out, err := os.CreateTemp(dir, ".vessel-replace-*")
	if err != nil {
		// Fallback: overwrite directly (non-atomic but still works)
		out, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	}
	tmpName := out.Name()
	defer func() {
		out.Close()
		os.Remove(tmpName)
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Chmod(0755); err != nil {
		return err
	}
	out.Close()
	return os.Rename(tmpName, dst)
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
