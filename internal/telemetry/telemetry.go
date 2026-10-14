// Package telemetry sends anonymous usage events to PostHog.
//
// It is opt-out: set DO_NOT_TRACK=1 or VESSEL_TELEMETRY=0 to turn it off. It is also off in CI
// and under `go test`. Events carry an anonymous random install id, the vessel
// version, OS and architecture, plus a few counts and enumerated values. Never names,
// domains, paths, environment variables, logs or any user content.
package telemetry

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// Public client key for the maintainer's PostHog project. It can only write events.
	posthogKey = "phc_oLm8fb7L4CXDqGdJcc7a6boCtR8rVjf6dNx7G995eGdy"
	endpoint   = "https://us.i.posthog.com/i/v0/e/"
	app        = "vessel"
	envVar     = "VESSEL_TELEMETRY"
	stateFile  = "telemetry.json"
)

const notice = `vessel collects anonymous usage data (version, OS, which features are used, and
whether they succeed) to guide development. It never sends names, domains, paths, logs or
configuration. Turn it off with VESSEL_TELEMETRY=0 or DO_NOT_TRACK=1.`

// Client sends events in the background. The zero value and nil are valid and disabled.
type Client struct {
	enabled    bool
	distinctID string
	version    string
	http       *http.Client
	wg         sync.WaitGroup
}

type state struct {
	InstallID string `json:"install_id"`
}

// Enabled reports whether telemetry is allowed by the environment.
func Enabled() bool {
	if isOn(os.Getenv("DO_NOT_TRACK")) || isOff(os.Getenv(envVar)) {
		return false
	}
	if isOn(os.Getenv("CI")) || testing.Testing() {
		return false
	}
	return true
}

func isOff(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "off", "no":
		return true
	}
	return false
}

func isOn(v string) bool { return strings.TrimSpace(v) != "" && !isOff(v) }

// New returns a client keyed by an install id stored in the first writable dir.
// It prints a one-time notice to stderr when it creates the id. If telemetry is
// disabled or no id can be stored, the client is a no-op.
func New(version string, dirs ...string) *Client {
	c := &Client{version: version, http: &http.Client{Timeout: 5 * time.Second}}
	if !Enabled() {
		return c
	}
	if userDir, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(userDir, app))
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if id := readID(dir); id != "" {
			c.distinctID, c.enabled = id, true
			return c
		}
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if id, err := createID(dir); err == nil {
			fmt.Fprintf(os.Stderr, "\n%s\n\n", notice)
			c.distinctID, c.enabled = id, true
			return c
		}
	}
	return c
}

func readID(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		return ""
	}
	var s state
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	return s.InstallID
}

func createID(dir string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(state{InstallID: id}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, stateFile), append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return id, nil
}

// Capture sends one event in the background. It never blocks and never fails loudly.
func (c *Client) Capture(event string, props map[string]any) {
	if c == nil || !c.enabled {
		return
	}
	properties := map[string]any{
		"app":                     app,
		"version":                 c.version,
		"os":                      runtime.GOOS,
		"arch":                    runtime.GOARCH,
		"$process_person_profile": false,
	}
	for k, v := range props {
		properties[k] = v
	}
	body, err := json.Marshal(map[string]any{
		"api_key":     posthogKey,
		"event":       event,
		"distinct_id": c.distinctID,
		"properties":  properties,
	})
	if err != nil {
		return
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		resp, err := c.http.Post(endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
}

// Flush waits up to timeout for in-flight events. Call it before a short-lived command exits.
func (c *Client) Flush(timeout time.Duration) {
	if c == nil || !c.enabled {
		return
	}
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}
