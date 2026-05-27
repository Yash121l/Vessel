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

	cf, err := GenerateCompose(tmpl, deployment, nil)
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

	cf, err := GenerateCompose(tmpl, deployment, nil)
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

func TestGenerateComposeSkipsOptionalService(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:    "demo",
		Name:  "Demo",
		Image: "demo/app:latest",
		EnvVars: []registry.EnvVar{
			{Key: "DATABASE_HOST", Default: "demo-db"},
		},
		ExtraServices: []registry.ServiceDef{
			{Name: "demo-db", Image: "postgres:16", Optional: true, Role: "database"},
		},
	}
	deployment := &store.Deployment{
		Name: "demo-app",
		Env:  map[string]string{"DATABASE_HOST": "external-db.example.com"},
	}

	cf, err := GenerateCompose(tmpl, deployment, map[string]bool{"demo-db": true})
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}
	if _, ok := cf.Services["demo-app-demo-db"]; ok {
		t.Fatalf("skipped service was still generated")
	}
	if got := cf.Services["demo-app"].Environment["DATABASE_HOST"]; got != "external-db.example.com" {
		t.Fatalf("DATABASE_HOST = %q, want external host", got)
	}
}
