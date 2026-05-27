package deployment

// Property 5: Sidecar port round-trip through compose generator
// Validates: Requirements 2.5
//
// For any ServiceDef with a non-empty Ports slice, GenerateCompose must produce
// a ComposeFile where the corresponding service entry contains a ports list of
// the same length, with each entry formatted as "[host:]external:internal/protocol".

import (
	"fmt"
	"math/rand"
	"regexp"
	"testing"

	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
)

// portFormatRe matches the two valid port string formats produced by GenerateCompose:
//   - With domain (localhost-bound): "127.0.0.1:external:internal/protocol"
//   - Without domain (public):       "external:internal/protocol"
var portFormatRe = regexp.MustCompile(`^(127\.0\.0\.1:\d+:\d+/\w+|\d+:\d+/\w+)$`)

// randomPort generates a random registry.Port with valid internal/external port
// numbers (1–65535) and a protocol of either "tcp" or "udp".
func randomPort(rng *rand.Rand) registry.Port {
	protocols := []string{"tcp", "udp"}
	internal := 1 + rng.Intn(65535)
	external := 1 + rng.Intn(65535)
	proto := protocols[rng.Intn(len(protocols))]
	return registry.Port{
		Internal: internal,
		External: external,
		Protocol: proto,
	}
}

// randomPortDefaultProtocol generates a random registry.Port with an empty
// protocol to exercise the default-to-tcp code path.
func randomPortDefaultProtocol(rng *rand.Rand) registry.Port {
	internal := 1 + rng.Intn(65535)
	external := 1 + rng.Intn(65535)
	return registry.Port{
		Internal: internal,
		External: external,
		Protocol: "", // should default to "tcp"
	}
}

// randomPortExternalZero generates a random registry.Port with External=0 to
// exercise the "external defaults to internal" code path.
func randomPortExternalZero(rng *rand.Rand) registry.Port {
	internal := 1 + rng.Intn(65535)
	return registry.Port{
		Internal: internal,
		External: 0, // should default to internal
		Protocol: "tcp",
	}
}

// expectedPortString returns the port string that GenerateCompose should produce
// for a given port and domain setting.
func expectedPortString(p registry.Port, hasDomain bool) string {
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	external := p.External
	if external == 0 {
		external = p.Internal
	}
	if hasDomain {
		return fmt.Sprintf("127.0.0.1:%d:%d/%s", external, p.Internal, proto)
	}
	return fmt.Sprintf("%d:%d/%s", external, p.Internal, proto)
}

// TestProperty5_SidecarPortRoundTrip_WithDomain verifies Property 5 when a
// domain is set: sidecar ports must be bound to 127.0.0.1 and formatted as
// "127.0.0.1:external:internal/protocol".
//
// Validates: Requirements 2.5
func TestProperty5_SidecarPortRoundTrip_WithDomain(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < iterations; i++ {
		// Generate between 1 and 8 ports for the sidecar.
		n := 1 + rng.Intn(8)
		ports := make([]registry.Port, n)
		for j := 0; j < n; j++ {
			ports[j] = randomPort(rng)
		}

		tmpl := &registry.AppTemplate{
			ID:    "test-app",
			Name:  "Test App",
			Image: "test/app:latest",
			ExtraServices: []registry.ServiceDef{
				{
					Name:  "test-sidecar",
					Image: "test/sidecar:latest",
					Ports: ports,
				},
			},
		}
		deployment := &store.Deployment{
			Name:   "my-deployment",
			Domain: "app.example.com", // domain set → localhost binding
		}

		cf, err := GenerateCompose(tmpl, deployment, nil)
		if err != nil {
			t.Fatalf("iteration %d: GenerateCompose() error = %v", i, err)
		}

		sidecarName := "my-deployment-test-sidecar"
		svc, ok := cf.Services[sidecarName]
		if !ok {
			t.Fatalf("iteration %d: sidecar service %q not found in compose output", i, sidecarName)
		}

		// Length must match.
		if len(svc.Ports) != n {
			t.Fatalf("iteration %d: sidecar ports length = %d, want %d\nports input: %+v\nports output: %v",
				i, len(svc.Ports), n, ports, svc.Ports)
		}

		// Each entry must match the expected format and value.
		for j, p := range ports {
			want := expectedPortString(p, true)
			got := svc.Ports[j]
			if got != want {
				t.Fatalf("iteration %d, port %d: got %q, want %q\nport input: %+v",
					i, j, got, want, p)
			}
			if !portFormatRe.MatchString(got) {
				t.Fatalf("iteration %d, port %d: %q does not match expected format", i, j, got)
			}
		}
	}
}

// TestProperty5_SidecarPortRoundTrip_WithoutDomain verifies Property 5 when no
// domain is set: sidecar ports must be publicly bound and formatted as
// "external:internal/protocol".
//
// Validates: Requirements 2.5
func TestProperty5_SidecarPortRoundTrip_WithoutDomain(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(99))

	for i := 0; i < iterations; i++ {
		// Generate between 1 and 8 ports for the sidecar.
		n := 1 + rng.Intn(8)
		ports := make([]registry.Port, n)
		for j := 0; j < n; j++ {
			ports[j] = randomPort(rng)
		}

		tmpl := &registry.AppTemplate{
			ID:    "test-app",
			Name:  "Test App",
			Image: "test/app:latest",
			ExtraServices: []registry.ServiceDef{
				{
					Name:  "test-sidecar",
					Image: "test/sidecar:latest",
					Ports: ports,
				},
			},
		}
		deployment := &store.Deployment{
			Name: "my-deployment",
			// No domain → public binding
		}

		cf, err := GenerateCompose(tmpl, deployment, nil)
		if err != nil {
			t.Fatalf("iteration %d: GenerateCompose() error = %v", i, err)
		}

		sidecarName := "my-deployment-test-sidecar"
		svc, ok := cf.Services[sidecarName]
		if !ok {
			t.Fatalf("iteration %d: sidecar service %q not found in compose output", i, sidecarName)
		}

		// Length must match.
		if len(svc.Ports) != n {
			t.Fatalf("iteration %d: sidecar ports length = %d, want %d\nports input: %+v\nports output: %v",
				i, len(svc.Ports), n, ports, svc.Ports)
		}

		// Each entry must match the expected format and value.
		for j, p := range ports {
			want := expectedPortString(p, false)
			got := svc.Ports[j]
			if got != want {
				t.Fatalf("iteration %d, port %d: got %q, want %q\nport input: %+v",
					i, j, got, want, p)
			}
			if !portFormatRe.MatchString(got) {
				t.Fatalf("iteration %d, port %d: %q does not match expected format", i, j, got)
			}
		}
	}
}

// TestProperty5_SidecarPortRoundTrip_DefaultProtocol verifies that when a port
// has an empty Protocol field, GenerateCompose defaults it to "tcp".
//
// Validates: Requirements 2.5
func TestProperty5_SidecarPortRoundTrip_DefaultProtocol(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(7))

	for i := 0; i < iterations; i++ {
		n := 1 + rng.Intn(5)
		ports := make([]registry.Port, n)
		for j := 0; j < n; j++ {
			ports[j] = randomPortDefaultProtocol(rng)
		}

		tmpl := &registry.AppTemplate{
			ID:    "test-app",
			Name:  "Test App",
			Image: "test/app:latest",
			ExtraServices: []registry.ServiceDef{
				{
					Name:  "test-sidecar",
					Image: "test/sidecar:latest",
					Ports: ports,
				},
			},
		}
		deployment := &store.Deployment{Name: "my-deployment"}

		cf, err := GenerateCompose(tmpl, deployment, nil)
		if err != nil {
			t.Fatalf("iteration %d: GenerateCompose() error = %v", i, err)
		}

		sidecarName := "my-deployment-test-sidecar"
		svc, ok := cf.Services[sidecarName]
		if !ok {
			t.Fatalf("iteration %d: sidecar service %q not found", i, sidecarName)
		}

		if len(svc.Ports) != n {
			t.Fatalf("iteration %d: sidecar ports length = %d, want %d", i, len(svc.Ports), n)
		}

		for j, p := range ports {
			got := svc.Ports[j]
			// Protocol must default to "tcp".
			want := fmt.Sprintf("%d:%d/tcp", p.External, p.Internal)
			if got != want {
				t.Fatalf("iteration %d, port %d: got %q, want %q (expected default tcp protocol)", i, j, got, want)
			}
		}
	}
}

// TestProperty5_SidecarPortRoundTrip_ExternalDefaultsToInternal verifies that
// when External == 0, GenerateCompose uses Internal as the external port.
//
// Validates: Requirements 2.5
func TestProperty5_SidecarPortRoundTrip_ExternalDefaultsToInternal(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(13))

	for i := 0; i < iterations; i++ {
		n := 1 + rng.Intn(5)
		ports := make([]registry.Port, n)
		for j := 0; j < n; j++ {
			ports[j] = randomPortExternalZero(rng)
		}

		tmpl := &registry.AppTemplate{
			ID:    "test-app",
			Name:  "Test App",
			Image: "test/app:latest",
			ExtraServices: []registry.ServiceDef{
				{
					Name:  "test-sidecar",
					Image: "test/sidecar:latest",
					Ports: ports,
				},
			},
		}
		deployment := &store.Deployment{Name: "my-deployment"}

		cf, err := GenerateCompose(tmpl, deployment, nil)
		if err != nil {
			t.Fatalf("iteration %d: GenerateCompose() error = %v", i, err)
		}

		sidecarName := "my-deployment-test-sidecar"
		svc, ok := cf.Services[sidecarName]
		if !ok {
			t.Fatalf("iteration %d: sidecar service %q not found", i, sidecarName)
		}

		if len(svc.Ports) != n {
			t.Fatalf("iteration %d: sidecar ports length = %d, want %d", i, len(svc.Ports), n)
		}

		for j, p := range ports {
			got := svc.Ports[j]
			// External=0 → use Internal as both host and container port.
			want := fmt.Sprintf("%d:%d/tcp", p.Internal, p.Internal)
			if got != want {
				t.Fatalf("iteration %d, port %d: got %q, want %q (expected external to default to internal=%d)",
					i, j, got, want, p.Internal)
			}
		}
	}
}

// TestProperty5_SidecarPortRoundTrip_MultipleSidecars verifies that when a
// template has multiple sidecars, each sidecar's ports are independently
// round-tripped correctly.
//
// Validates: Requirements 2.5
func TestProperty5_SidecarPortRoundTrip_MultipleSidecars(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(55))

	for i := 0; i < iterations; i++ {
		// Generate 2–4 sidecars, each with 1–4 ports.
		numSidecars := 2 + rng.Intn(3)
		sidecars := make([]registry.ServiceDef, numSidecars)
		for s := 0; s < numSidecars; s++ {
			numPorts := 1 + rng.Intn(4)
			ports := make([]registry.Port, numPorts)
			for j := 0; j < numPorts; j++ {
				ports[j] = randomPort(rng)
			}
			sidecars[s] = registry.ServiceDef{
				Name:  fmt.Sprintf("sidecar-%d", s),
				Image: fmt.Sprintf("test/sidecar-%d:latest", s),
				Ports: ports,
			}
		}

		tmpl := &registry.AppTemplate{
			ID:            "test-app",
			Name:          "Test App",
			Image:         "test/app:latest",
			ExtraServices: sidecars,
		}
		deployment := &store.Deployment{Name: "my-deployment"}

		cf, err := GenerateCompose(tmpl, deployment, nil)
		if err != nil {
			t.Fatalf("iteration %d: GenerateCompose() error = %v", i, err)
		}

		for s, sidecar := range sidecars {
			svcName := fmt.Sprintf("my-deployment-%s", sidecar.Name)
			svc, ok := cf.Services[svcName]
			if !ok {
				t.Fatalf("iteration %d, sidecar %d: service %q not found in compose output", i, s, svcName)
			}

			if len(svc.Ports) != len(sidecar.Ports) {
				t.Fatalf("iteration %d, sidecar %d: ports length = %d, want %d",
					i, s, len(svc.Ports), len(sidecar.Ports))
			}

			for j, p := range sidecar.Ports {
				want := expectedPortString(p, false)
				got := svc.Ports[j]
				if got != want {
					t.Fatalf("iteration %d, sidecar %d, port %d: got %q, want %q",
						i, s, j, got, want)
				}
				if !portFormatRe.MatchString(got) {
					t.Fatalf("iteration %d, sidecar %d, port %d: %q does not match expected format",
						i, s, j, got)
				}
			}
		}
	}
}
