package telemetry

import "testing"

func TestDisabledUnderTests(t *testing.T) {
	if Enabled() {
		t.Fatal("telemetry must be off under go test")
	}
	c := New("test", t.TempDir())
	c.Capture("should_not_send", nil)
	c.Flush(0)
}

func TestOptOutValues(t *testing.T) {
	for _, v := range []string{"0", "false", "OFF", "no"} {
		if !isOff(v) {
			t.Errorf("isOff(%q) = false", v)
		}
	}
	for _, v := range []string{"1", "true", "yes"} {
		if !isOn(v) {
			t.Errorf("isOn(%q) = false", v)
		}
	}
	if isOn("") || isOn("0") {
		t.Error("empty and 0 must not count as on")
	}
}

func TestNilClientIsSafe(t *testing.T) {
	var c *Client
	c.Capture("x", nil)
	c.Flush(0)
}
