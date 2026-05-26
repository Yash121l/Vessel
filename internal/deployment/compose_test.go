package deployment

import (
	"strings"
	"testing"

	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
)

func TestGenerateComposeBindsDomainPortsToLocalhost(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:        "demo",
		Name:      "Demo",
		Image:     "nginx:1.27",
		ProxyPort: 8080,
		Ports: []registry.Port{
			{Internal: 8080, External: 18080, Protocol: "tcp"},
		},
	}
	deployment := &store.Deployment{Name: "demo-app", Domain: "demo.example.com"}

	cf, err := GenerateCompose(tmpl, deployment)
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}

	ports := cf.Services["demo-app"].Ports
	if len(ports) != 1 || ports[0] != "127.0.0.1:18080:8080/tcp" {
		t.Fatalf("ports = %#v, want localhost-bound proxy port", ports)
	}
}

func TestGenerateComposePublishesPortsWithoutDomain(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:    "demo",
		Name:  "Demo",
		Image: "nginx:1.27",
		Ports: []registry.Port{
			{Internal: 8080, External: 18080, Protocol: "tcp"},
		},
	}
	deployment := &store.Deployment{Name: "demo-app"}

	cf, err := GenerateCompose(tmpl, deployment)
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}

	ports := strings.Join(cf.Services["demo-app"].Ports, ",")
	if ports != "18080:8080/tcp" {
		t.Fatalf("ports = %q, want public host binding", ports)
	}
}

func TestProxyTargetPortUsesExternalPortForCaddy(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ProxyPort: 3000,
		Ports: []registry.Port{
			{Internal: 3000, External: 13000, Protocol: "tcp"},
		},
	}
	if got := proxyTargetPort(tmpl); got != 13000 {
		t.Fatalf("proxyTargetPort() = %d, want 13000", got)
	}
}
