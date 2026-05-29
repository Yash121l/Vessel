package deployment

import "testing"

func TestParseComposeDetailOutputJSONArray(t *testing.T) {
	out := []byte(`[
		{
			"Name":"umami-umami-1",
			"Image":"ghcr.io/umami-software/umami:postgresql-latest",
			"State":"running",
			"CreatedAt":"2026-05-29 14:19:25 +0000 UTC",
			"Publishers":[{"URL":"127.0.0.1","TargetPort":3000,"PublishedPort":3001,"Protocol":"tcp"}]
		},
		{
			"Name":"umami-umami-db-1",
			"Image":"postgres:15-alpine",
			"State":"running",
			"CreatedAt":"2026-05-29 14:19:25 +0000 UTC",
			"Publishers":[{"TargetPort":5432,"Protocol":"tcp"}]
		}
	]`)

	services, ok := parseComposeDetailOutput(out)
	if !ok {
		t.Fatal("parseComposeDetailOutput returned ok=false for valid JSON array")
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	if services[0].Name != "umami-umami-1" {
		t.Fatalf("unexpected first service name: %q", services[0].Name)
	}
	if services[0].State != "running" {
		t.Fatalf("unexpected first service state: %q", services[0].State)
	}
	if services[0].Ports != "127.0.0.1:3001->3000/tcp" {
		t.Fatalf("unexpected first service ports: %q", services[0].Ports)
	}
	if services[1].Ports != "5432/tcp" {
		t.Fatalf("unexpected second service ports: %q", services[1].Ports)
	}
}

func TestParseComposeDetailOutputNDJSON(t *testing.T) {
	out := []byte("{\"Name\":\"api\",\"Image\":\"ghcr.io/example/api:latest\",\"Status\":\"running (healthy)\",\"Ports\":\"0.0.0.0:8080->8080/tcp\",\"CreatedAt\":\"2026-05-29 14:19:25 +0000 UTC\"}\n" +
		"{\"Name\":\"worker\",\"Image\":\"ghcr.io/example/worker:latest\",\"State\":\"exited\",\"CreatedAt\":\"2026-05-29 14:20:00 +0000 UTC\"}\n")

	services, ok := parseComposeDetailOutput(out)
	if !ok {
		t.Fatal("parseComposeDetailOutput returned ok=false for valid NDJSON")
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].State != "running" {
		t.Fatalf("expected status fallback to produce running, got %q", services[0].State)
	}
	if services[0].Ports != "0.0.0.0:8080->8080/tcp" {
		t.Fatalf("unexpected first service ports: %q", services[0].Ports)
	}
	if services[1].State != "exited" {
		t.Fatalf("unexpected second service state: %q", services[1].State)
	}
}
