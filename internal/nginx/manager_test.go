package nginx

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCreateSiteForDeployment(t *testing.T) {
	// Set up a temporary nginx config root with a sites-available directory.
	tmpDir := t.TempDir()
	sitesAvailable := filepath.Join(tmpDir, "sites-available")
	sitesEnabled := filepath.Join(tmpDir, "sites-enabled")
	if err := os.MkdirAll(sitesAvailable, 0755); err != nil {
		t.Fatalf("failed to create sites-available: %v", err)
	}
	if err := os.MkdirAll(sitesEnabled, 0755); err != nil {
		t.Fatalf("failed to create sites-enabled: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "nginx.conf"), []byte("include /etc/nginx/sites-enabled/*;"), 0644); err != nil {
		t.Fatalf("failed to create nginx.conf: %v", err)
	}

	m := &Manager{configRoot: tmpDir}

	tests := []struct {
		name           string
		siteName       string
		serverName     string
		port           int
		upstream       string
		deploymentName string
	}{
		{
			name:           "basic deployment comment prepended",
			siteName:       "myapp",
			serverName:     "myapp.example.com",
			port:           8080,
			upstream:       "",
			deploymentName: "myapp",
		},
		{
			name:           "deployment name with hyphens",
			siteName:       "my-service",
			serverName:     "service.example.com",
			port:           3000,
			upstream:       "localhost:3000",
			deploymentName: "my-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.CreateSiteForDeployment(tt.siteName, tt.serverName, tt.port, tt.upstream, tt.deploymentName)
			if err != nil {
				t.Fatalf("CreateSiteForDeployment returned error: %v", err)
			}

			// Read the written file.
			data, err := os.ReadFile(filepath.Join(sitesAvailable, tt.siteName))
			if err != nil {
				t.Fatalf("failed to read written site file: %v", err)
			}
			content := string(data)

			// Verify the first line is the vessel-deployment comment.
			expectedFirstLine := "# vessel-deployment: " + tt.deploymentName
			lines := strings.SplitN(content, "\n", 2)
			if lines[0] != expectedFirstLine {
				t.Errorf("first line = %q, want %q", lines[0], expectedFirstLine)
			}

			// Verify extractVesselDeployment round-trips correctly.
			extracted := extractVesselDeployment(content)
			if extracted != tt.deploymentName {
				t.Errorf("extractVesselDeployment = %q, want %q", extracted, tt.deploymentName)
			}

			// Verify the rest of the content is a valid nginx config (contains server block).
			if !strings.Contains(content, "server {") {
				t.Errorf("content does not contain server block: %q", content)
			}
			if !strings.Contains(content, tt.serverName) {
				t.Errorf("content does not contain server_name %q", tt.serverName)
			}
			if !strings.Contains(content, "location /.well-known/acme-challenge/") {
				t.Errorf("content does not contain ACME challenge location: %q", content)
			}
			if !strings.Contains(content, "proxy_pass http://127.0.0.1:") && tt.upstream == "" {
				t.Errorf("content does not proxy to localhost loopback: %q", content)
			}
		})
	}
}

func TestBuildSiteConfigUsesExplicitUpstream(t *testing.T) {
	content := buildSiteConfig("app.example.com", 8080, "unix:/run/app.sock")
	if !strings.Contains(content, "proxy_pass http://unix:/run/app.sock;") {
		t.Fatalf("buildSiteConfig did not use explicit upstream: %q", content)
	}
}

func TestSaveSiteDefaultsToConfDWhenSitesLayoutMissing(t *testing.T) {
	tmpDir := t.TempDir()
	confD := filepath.Join(tmpDir, "conf.d")
	if err := os.MkdirAll(confD, 0755); err != nil {
		t.Fatalf("failed to create conf.d: %v", err)
	}

	m := &Manager{configRoot: tmpDir}
	if err := m.SaveSite("demo", "server {}"); err != nil {
		t.Fatalf("SaveSite() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(confD, "demo.conf"))
	if err != nil {
		t.Fatalf("failed to read conf.d site file: %v", err)
	}
	if string(data) != "server {}" {
		t.Fatalf("saved site content = %q, want %q", string(data), "server {}")
	}
}

func TestRestartFallbackStopsThenStartsNginx(t *testing.T) {
	origRunCommand := runCommand
	defer func() { runCommand = origRunCommand }()

	var calls []string
	runCommand = func(name string, args ...string) error {
		call := name
		if len(args) > 0 {
			call += " " + strings.Join(args, " ")
		}
		calls = append(calls, call)
		if name == "systemctl" && len(args) == 2 && args[0] == "restart" && args[1] == "nginx" {
			return fmt.Errorf("systemctl unavailable")
		}
		return nil
	}

	m := &Manager{}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	want := []string{
		"systemctl restart nginx",
		"nginx -s stop",
		"nginx",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("Restart() calls = %#v, want %#v", calls, want)
	}
}

func TestExtractVesselDeployment(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "no comment returns empty string",
			content:  "server {\n    listen 80;\n    server_name example.com;\n}\n",
			expected: "",
		},
		{
			name:     "comment on first line",
			content:  "# vessel-deployment: myapp\nserver {\n    listen 80;\n}\n",
			expected: "myapp",
		},
		{
			name:     "comment on non-first line still extracts correctly",
			content:  "# some other comment\n# vessel-deployment: myapp\nserver {\n    listen 80;\n}\n",
			expected: "myapp",
		},
		{
			name:     "deployment name with spaces",
			content:  "# vessel-deployment: my deployment\nserver {}\n",
			expected: "my deployment",
		},
		{
			name:     "deployment name with leading/trailing whitespace trimmed",
			content:  "#   vessel-deployment:   trimmed-name   \nserver {}\n",
			expected: "",
		},
		{
			name:     "comment embedded in config body is still found",
			content:  "server {\n    listen 80;\n    # vessel-deployment: embedded\n    server_name x.com;\n}\n",
			expected: "embedded",
		},
		{
			name:     "empty content returns empty string",
			content:  "",
			expected: "",
		},
		{
			name:     "partial prefix does not match",
			content:  "# vessel-deploy: notthis\nserver {}\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVesselDeployment(tt.content)
			if got != tt.expected {
				t.Errorf("extractVesselDeployment(%q) = %q, want %q", tt.content, got, tt.expected)
			}
		})
	}
}
