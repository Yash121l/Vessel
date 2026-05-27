package deployment

// Property 10: SyncStatus running detection
// Validates: Requirements 5.2
//
// For any JSON array output from `docker compose ps --format json` where at
// least one element has State == "running", parseSyncStatusOutput must return
// status "running".

import (
	"encoding/json"
	"math/rand"
	"testing"
)

// serviceEntry mirrors the JSON shape produced by docker compose ps --format json.
type serviceEntry struct {
	Name  string `json:"Name"`
	State string `json:"State"`
}

// nonRunningStates is the pool of states that are NOT "running".
var nonRunningStates = []string{
	"exited", "stopped", "created", "paused", "restarting", "dead", "removing", "",
}

// randomNonRunningState returns a random state that is not "running".
func randomNonRunningState(rng *rand.Rand) string {
	return nonRunningStates[rng.Intn(len(nonRunningStates))]
}

// buildJSONArray serialises a slice of serviceEntry values to a JSON array.
func buildJSONArray(entries []serviceEntry) []byte {
	data, _ := json.Marshal(entries)
	return data
}

// buildNDJSON serialises a slice of serviceEntry values as NDJSON (one object per line).
func buildNDJSON(entries []serviceEntry) []byte {
	var out []byte
	for i, e := range entries {
		line, _ := json.Marshal(e)
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, line...)
	}
	return out
}

// TestProperty10_SyncStatusRunningDetection_JSONArray verifies Property 10 using
// JSON array format: for any array with at least one "running" service,
// parseSyncStatusOutput must return "running".
//
// Validates: Requirements 5.2
func TestProperty10_SyncStatusRunningDetection_JSONArray(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < iterations; i++ {
		// Generate between 1 and 10 services.
		n := 1 + rng.Intn(10)
		entries := make([]serviceEntry, n)

		// Pick a random index to be the "running" service.
		runningIdx := rng.Intn(n)

		for j := 0; j < n; j++ {
			name := "svc"
			if j == runningIdx {
				entries[j] = serviceEntry{Name: name, State: "running"}
			} else {
				entries[j] = serviceEntry{Name: name, State: randomNonRunningState(rng)}
			}
		}

		out := buildJSONArray(entries)
		status, ok := parseSyncStatusOutput(out)

		if !ok {
			t.Fatalf("iteration %d: parseSyncStatusOutput returned ok=false for valid JSON array: %s", i, out)
		}
		if status != "running" {
			t.Fatalf("iteration %d: expected status \"running\" (at least one service is running), got %q\ninput: %s", i, status, out)
		}
	}
}

// TestProperty10_SyncStatusRunningDetection_NDJSON verifies Property 10 using
// NDJSON format: for any NDJSON output with at least one "running" service,
// parseSyncStatusOutput must return "running".
//
// Validates: Requirements 5.2
func TestProperty10_SyncStatusRunningDetection_NDJSON(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(99))

	for i := 0; i < iterations; i++ {
		// Generate between 1 and 10 services.
		n := 1 + rng.Intn(10)
		entries := make([]serviceEntry, n)

		// Pick a random index to be the "running" service.
		runningIdx := rng.Intn(n)

		for j := 0; j < n; j++ {
			name := "svc"
			if j == runningIdx {
				entries[j] = serviceEntry{Name: name, State: "running"}
			} else {
				entries[j] = serviceEntry{Name: name, State: randomNonRunningState(rng)}
			}
		}

		out := buildNDJSON(entries)
		status, ok := parseSyncStatusOutput(out)

		if !ok {
			t.Fatalf("iteration %d: parseSyncStatusOutput returned ok=false for valid NDJSON: %s", i, out)
		}
		if status != "running" {
			t.Fatalf("iteration %d: expected status \"running\" (at least one service is running), got %q\ninput: %s", i, status, out)
		}
	}
}

// TestProperty10_SyncStatusRunningDetection_MultipleRunning verifies that when
// multiple services are "running", the result is still "running".
//
// Validates: Requirements 5.2
func TestProperty10_SyncStatusRunningDetection_MultipleRunning(t *testing.T) {
	const iterations = 20
	rng := rand.New(rand.NewSource(7))

	for i := 0; i < iterations; i++ {
		// Generate between 2 and 10 services, all "running".
		n := 2 + rng.Intn(9)
		entries := make([]serviceEntry, n)
		for j := 0; j < n; j++ {
			entries[j] = serviceEntry{Name: "svc", State: "running"}
		}

		out := buildJSONArray(entries)
		status, ok := parseSyncStatusOutput(out)

		if !ok {
			t.Fatalf("iteration %d: parseSyncStatusOutput returned ok=false for valid JSON array: %s", i, out)
		}
		if status != "running" {
			t.Fatalf("iteration %d: expected status \"running\" (all services running), got %q\ninput: %s", i, status, out)
		}
	}
}
