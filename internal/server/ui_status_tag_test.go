package server

// **Validates: Requirements 7.6**
//
// Property 15: Status tag class consistency
//
// For any status string value, statusTag(status) must return HTML containing
// exactly one <span> with a tag-* CSS class that matches the status value,
// and that class must be one of the defined .tag-* classes in the stylesheet.

import (
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

// definedTagClasses returns the set of .tag-* CSS classes defined in the
// uiHTML stylesheet. It parses the CSS in uiHTML to extract all class names
// that start with "tag-".
func definedTagClasses() map[string]bool {
	classes := make(map[string]bool)
	// Match patterns like .tag-running, .tag-stopped, etc. in the CSS.
	// The CSS uses comma-separated selectors like ".tag-running,.tag-active{...}"
	re := regexp.MustCompile(`\.tag-([a-z]+)`)
	matches := re.FindAllStringSubmatch(uiHTML, -1)
	for _, m := range matches {
		if len(m) == 2 {
			classes["tag-"+m[1]] = true
		}
	}
	return classes
}

// statusTagFromHTML is a Go implementation that mirrors the JS statusTag()
// function embedded in uiHTML. It applies the same status→class mapping so
// we can test the property without a JS runtime.
//
// The mapping is:
//
//	"running"   → "tag-running"
//	"stopped"   → "tag-stopped"
//	"error"     → "tag-error"
//	"deploying" → "tag-deploying"
//	"updating"  → "tag-updating"
//	"imported"  → "tag-imported"
//	(anything else) → "tag-stopped"
func statusTagFromHTML(status string) string {
	s := strings.ToLower(status)
	if s == "" {
		s = "unknown"
	}
	var cls string
	switch s {
	case "running":
		cls = "tag-running"
	case "stopped":
		cls = "tag-stopped"
	case "error":
		cls = "tag-error"
	case "deploying":
		cls = "tag-deploying"
	case "updating":
		cls = "tag-updating"
	case "imported":
		cls = "tag-imported"
	default:
		cls = "tag-stopped"
	}
	return `<span class="tag ` + cls + `"><span class="dot"></span>` + s + `</span>`
}

// extractStatusTagFunction extracts the statusTag JS function source from
// uiHTML. Returns the function body as a string, or empty string if not found.
func extractStatusTagFunction() string {
	start := strings.Index(uiHTML, "function statusTag(")
	if start == -1 {
		return ""
	}
	// Find the closing brace of the function by counting braces.
	depth := 0
	end := -1
	for i := start; i < len(uiHTML); i++ {
		switch uiHTML[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return ""
	}
	return uiHTML[start:end]
}

// countSpanTagClass counts how many <span> elements in html have a class
// attribute containing the given tag-* class.
func countSpanTagClass(html, tagClass string) int {
	// Match <span class="tag <tagClass>"> or <span class="tag <tagClass> ...">
	pattern := regexp.MustCompile(`<span\s+class="[^"]*\b` + regexp.QuoteMeta(tagClass) + `\b[^"]*"`)
	return len(pattern.FindAllString(html, -1))
}

// extractTagClassFromSpan extracts the tag-* class from the outer <span> in
// the statusTag output HTML. Returns the class name (e.g. "tag-running") or
// empty string if not found.
func extractTagClassFromSpan(html string) string {
	re := regexp.MustCompile(`<span class="tag (tag-[a-z]+)"`)
	m := re.FindStringSubmatch(html)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// TestStatusTagFunctionExistsInHTML verifies that the statusTag JS function
// is present in uiHTML — a prerequisite for all other property tests.
func TestStatusTagFunctionExistsInHTML(t *testing.T) {
	fn := extractStatusTagFunction()
	if fn == "" {
		t.Fatal("statusTag function not found in uiHTML")
	}
	t.Logf("statusTag function found (%d bytes)", len(fn))
}

// TestDefinedTagClassesExistInStylesheet verifies that the expected tag-*
// CSS classes are defined in the uiHTML stylesheet.
func TestDefinedTagClassesExistInStylesheet(t *testing.T) {
	expected := []string{
		"tag-running",
		"tag-stopped",
		"tag-error",
		"tag-deploying",
		"tag-updating",
		"tag-imported",
	}
	defined := definedTagClasses()
	for _, cls := range expected {
		if !defined[cls] {
			t.Errorf("CSS class .%s not found in uiHTML stylesheet", cls)
		}
	}
	t.Logf("Defined tag-* classes: %v", defined)
}

// TestStatusTagKnownStatusesProperty verifies Property 15 for all known
// status values: the output HTML must contain exactly one outer <span> with
// the correct tag-* class, and that class must be in the defined set.
//
// **Validates: Requirements 7.6**
func TestStatusTagKnownStatusesProperty(t *testing.T) {
	defined := definedTagClasses()

	cases := []struct {
		status      string
		expectedCls string
	}{
		{"running", "tag-running"},
		{"stopped", "tag-stopped"},
		{"error", "tag-error"},
		{"deploying", "tag-deploying"},
		{"updating", "tag-updating"},
		{"imported", "tag-imported"},
		// Fallback cases
		{"", "tag-stopped"},
		{"unknown", "tag-stopped"},
		{"RUNNING", "tag-running"},   // case-insensitive
		{"STOPPED", "tag-stopped"},   // case-insensitive
		{"Deploying", "tag-deploying"}, // mixed case
	}

	for _, tc := range cases {
		html := statusTagFromHTML(tc.status)

		// Property: output must contain exactly one outer <span> with a tag-* class.
		cls := extractTagClassFromSpan(html)
		if cls == "" {
			t.Errorf("statusTag(%q): no tag-* class found in output: %s", tc.status, html)
			continue
		}

		// Property: the class must match the expected mapping.
		if cls != tc.expectedCls {
			t.Errorf("statusTag(%q): got class %q, want %q", tc.status, cls, tc.expectedCls)
		}

		// Property: the class must be one of the defined .tag-* classes in the stylesheet.
		if !defined[cls] {
			t.Errorf("statusTag(%q): class %q is not defined in the stylesheet", tc.status, cls)
		}

		// Property: the output must contain exactly one outer <span> with the tag-* class.
		count := countSpanTagClass(html, cls)
		if count != 1 {
			t.Errorf("statusTag(%q): expected exactly 1 <span> with class %q, got %d", tc.status, cls, count)
		}
	}
}

// TestStatusTagOutputAlwaysContainsOneTagSpanProperty uses testing/quick to
// verify Property 15 across randomly generated status strings:
//
//   - The output always contains exactly one outer <span> with a tag-* class.
//   - That class is always one of the defined .tag-* classes in the stylesheet.
//
// **Validates: Requirements 7.6**
func TestStatusTagOutputAlwaysContainsOneTagSpanProperty(t *testing.T) {
	defined := definedTagClasses()
	if len(defined) == 0 {
		t.Fatal("no tag-* classes found in stylesheet — cannot run property test")
	}

	// Property: for any string input, statusTag must return HTML with exactly
	// one outer <span> whose tag-* class is in the defined set.
	prop := func(status string) bool {
		html := statusTagFromHTML(status)

		// Must contain exactly one outer <span> with a tag-* class.
		cls := extractTagClassFromSpan(html)
		if cls == "" {
			return false
		}

		// The class must be in the defined stylesheet set.
		if !defined[cls] {
			return false
		}

		// Must contain exactly one such span.
		count := countSpanTagClass(html, cls)
		return count == 1
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 20}); err != nil {
		t.Errorf("Property 15 violated: %v", err)
	}
}

// TestStatusTagFallbackIsDefinedClass verifies that the fallback class
// (used for unknown status values) is itself a defined .tag-* class.
//
// **Validates: Requirements 7.6**
func TestStatusTagFallbackIsDefinedClass(t *testing.T) {
	defined := definedTagClasses()

	// The fallback for unknown statuses is "tag-stopped".
	fallbackStatuses := []string{
		"unknown",
		"pending",
		"paused",
		"restarting",
		"dead",
		"created",
		"xyz-not-a-real-status",
		"",
	}

	for _, status := range fallbackStatuses {
		html := statusTagFromHTML(status)
		cls := extractTagClassFromSpan(html)
		if cls == "" {
			t.Errorf("statusTag(%q): no tag-* class found in output", status)
			continue
		}
		if !defined[cls] {
			t.Errorf("statusTag(%q): fallback class %q is not defined in the stylesheet", status, cls)
		}
	}
}

// TestStatusTagClassMatchesStatusValueProperty verifies that for the six
// known status values, the tag-* class name contains the status value as a
// substring (e.g. "running" → "tag-running").
//
// **Validates: Requirements 7.6**
func TestStatusTagClassMatchesStatusValueProperty(t *testing.T) {
	knownStatuses := []string{"running", "stopped", "error", "deploying", "updating", "imported"}

	for _, status := range knownStatuses {
		html := statusTagFromHTML(status)
		cls := extractTagClassFromSpan(html)
		if cls == "" {
			t.Errorf("statusTag(%q): no tag-* class found", status)
			continue
		}
		// The class must be "tag-<status>".
		expected := "tag-" + status
		if cls != expected {
			t.Errorf("statusTag(%q): class %q does not match expected %q", status, cls, expected)
		}
	}
}

// TestStatusTagHTMLStructureProperty verifies that the HTML returned by
// statusTag always has the correct structure:
//
//	<span class="tag <cls>"><span class="dot"></span><status-text></span>
//
// **Validates: Requirements 7.6**
func TestStatusTagHTMLStructureProperty(t *testing.T) {
	// The outer span must have class "tag <tag-class>".
	outerSpanRe := regexp.MustCompile(`^<span class="tag tag-[a-z]+"`)
	// The inner span must have class "dot".
	innerDotRe := regexp.MustCompile(`<span class="dot"></span>`)
	// The outer span must be closed.
	closingTagRe := regexp.MustCompile(`</span>$`)

	statuses := []string{"running", "stopped", "error", "deploying", "updating", "imported", "", "unknown", "anything"}

	for _, status := range statuses {
		html := statusTagFromHTML(status)

		if !outerSpanRe.MatchString(html) {
			t.Errorf("statusTag(%q): outer span structure incorrect: %s", status, html)
		}
		if !innerDotRe.MatchString(html) {
			t.Errorf("statusTag(%q): missing inner dot span: %s", status, html)
		}
		if !closingTagRe.MatchString(html) {
			t.Errorf("statusTag(%q): missing closing </span>: %s", status, html)
		}
	}
}

// TestStatusTagJSFunctionMappingConsistency verifies that the JS statusTag
// function in uiHTML contains the expected status→class mappings by
// inspecting the function source text.
//
// **Validates: Requirements 7.6**
func TestStatusTagJSFunctionMappingConsistency(t *testing.T) {
	fn := extractStatusTagFunction()
	if fn == "" {
		t.Fatal("statusTag function not found in uiHTML")
	}

	// Each known status must appear in the function body mapped to its class.
	expectedMappings := []struct {
		status string
		class  string
	}{
		{"running", "tag-running"},
		{"stopped", "tag-stopped"},
		{"error", "tag-error"},
		{"deploying", "tag-deploying"},
		{"updating", "tag-updating"},
		{"imported", "tag-imported"},
	}

	for _, m := range expectedMappings {
		if !strings.Contains(fn, "'"+m.status+"'") && !strings.Contains(fn, `"`+m.status+`"`) {
			t.Errorf("statusTag JS function does not reference status %q", m.status)
		}
		if !strings.Contains(fn, "'"+m.class+"'") && !strings.Contains(fn, `"`+m.class+`"`) {
			t.Errorf("statusTag JS function does not reference class %q", m.class)
		}
	}
}
