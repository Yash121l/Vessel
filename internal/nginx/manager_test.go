package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSiteForDeployment(t *testing.T) {
	// Set up a temporary nginx config root with a sites-available directory.
	tmpDir := t.TempDir()
	sitesAvailable := filepath.Join(tmpDir, "sites-available")
	if err := os.MkdirAll(sitesAvailable, 0755); err != nil {
		t.Fatalf("failed to create sites-available: %v", err)
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
		})
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
