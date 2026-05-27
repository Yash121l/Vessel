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

func TestGenerateComposeSidecarPortsWithDomain(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:    "demo",
		Name:  "Demo",
		Image: "demo/app:latest",
		ExtraServices: []registry.ServiceDef{
			{
				Name:  "demo-db",
				Image: "postgres:16",
				Ports: []registry.Port{
					{Internal: 5432, External: 15432, Protocol: "tcp"},
				},
			},
		},
	}
	deployment := &store.Deployment{Name: "demo-app", Domain: "demo.example.com"}

	cf, err := GenerateCompose(tmpl, deployment, nil)
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}

	sidecar, ok := cf.Services["demo-app-demo-db"]
	if !ok {
		t.Fatal("sidecar service not found in compose output")
	}
	if len(sidecar.Ports) != 1 || sidecar.Ports[0] != "127.0.0.1:15432:5432/tcp" {
		t.Fatalf("sidecar ports = %#v, want localhost-bound port", sidecar.Ports)
	}
}

func TestGenerateComposeSidecarPortsWithoutDomain(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:    "demo",
		Name:  "Demo",
		Image: "demo/app:latest",
		ExtraServices: []registry.ServiceDef{
			{
				Name:  "demo-db",
				Image: "postgres:16",
				Ports: []registry.Port{
					{Internal: 5432, External: 15432, Protocol: "tcp"},
				},
			},
		},
	}
	deployment := &store.Deployment{Name: "demo-app"}

	cf, err := GenerateCompose(tmpl, deployment, nil)
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}

	sidecar, ok := cf.Services["demo-app-demo-db"]
	if !ok {
		t.Fatal("sidecar service not found in compose output")
	}
	if len(sidecar.Ports) != 1 || sidecar.Ports[0] != "15432:5432/tcp" {
		t.Fatalf("sidecar ports = %#v, want public host binding", sidecar.Ports)
	}
}

func TestGenerateComposeSidecarPortsDefaultProtocol(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:    "demo",
		Name:  "Demo",
		Image: "demo/app:latest",
		ExtraServices: []registry.ServiceDef{
			{
				Name:  "demo-cache",
				Image: "redis:7",
				Ports: []registry.Port{
					{Internal: 6379, External: 6379}, // no protocol set
				},
			},
		},
	}
	deployment := &store.Deployment{Name: "demo-app"}

	cf, err := GenerateCompose(tmpl, deployment, nil)
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}

	sidecar, ok := cf.Services["demo-app-demo-cache"]
	if !ok {
		t.Fatal("sidecar service not found in compose output")
	}
	if len(sidecar.Ports) != 1 || sidecar.Ports[0] != "6379:6379/tcp" {
		t.Fatalf("sidecar ports = %#v, want default tcp protocol", sidecar.Ports)
	}
}

func TestGenerateComposeSidecarPortsExternalDefaultsToInternal(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:    "demo",
		Name:  "Demo",
		Image: "demo/app:latest",
		ExtraServices: []registry.ServiceDef{
			{
				Name:  "demo-db",
				Image: "postgres:16",
				Ports: []registry.Port{
					{Internal: 5432, External: 0, Protocol: "tcp"}, // external=0 → use internal
				},
			},
		},
	}
	deployment := &store.Deployment{Name: "demo-app"}

	cf, err := GenerateCompose(tmpl, deployment, nil)
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}

	sidecar, ok := cf.Services["demo-app-demo-db"]
	if !ok {
		t.Fatal("sidecar service not found in compose output")
	}
	if len(sidecar.Ports) != 1 || sidecar.Ports[0] != "5432:5432/tcp" {
		t.Fatalf("sidecar ports = %#v, want external defaulting to internal port", sidecar.Ports)
	}
}

func TestGenerateComposeSidecarNoPortsWhenEmpty(t *testing.T) {
	tmpl := &registry.AppTemplate{
		ID:    "demo",
		Name:  "Demo",
		Image: "demo/app:latest",
		ExtraServices: []registry.ServiceDef{
			{
				Name:  "demo-db",
				Image: "postgres:16",
				// No Ports defined
			},
		},
	}
	deployment := &store.Deployment{Name: "demo-app"}

	cf, err := GenerateCompose(tmpl, deployment, nil)
	if err != nil {
		t.Fatalf("GenerateCompose() error = %v", err)
	}

	sidecar, ok := cf.Services["demo-app-demo-db"]
	if !ok {
		t.Fatal("sidecar service not found in compose output")
	}
	if len(sidecar.Ports) != 0 {
		t.Fatalf("sidecar ports = %#v, want empty ports slice", sidecar.Ports)
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
