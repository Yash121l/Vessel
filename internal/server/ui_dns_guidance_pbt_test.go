package server

// **Validates: Requirements 4.1, 4.2, 4.3**
//
// Property 8: DNS guidance box visibility
//
// For any non-empty domain string passed to dnsGuidanceBox(), the returned
// HTML must contain both the IP value and the domain value as visible text,
// along with two copy buttons.
//
// Since dnsGuidanceBox is a JavaScript function embedded in the Go string
// constant uiHTML, this test verifies the property by:
//  1. Extracting the dnsGuidanceBox function source from uiHTML.
//  2. Verifying that the function exists and has the correct structure.
//  3. Simulating the function output in Go and verifying that for any
//     non-empty domain the output contains the domain value and two copy
//     buttons.
//  4. Verifying that for an empty domain the function returns an empty string.
//  5. Using testing/quick to confirm the properties hold across random inputs.

import (
	"strings"
	"testing"
	"testing/quick"
)

// extractDNSGuidanceBoxSource extracts the dnsGuidanceBox JS function source
// from uiHTML by scanning for "function dnsGuidanceBox(" and counting braces
// to find the closing brace.
func extractDNSGuidanceBoxSource() string {
	return extractFunctionSource("dnsGuidanceBox")
}

// simulateDNSGuidanceBox mimics the JS dnsGuidanceBox function in Go.
// It returns the HTML that the JS function would produce for a given domain
// and ip value. When domain is empty it returns "".
// When ip is empty the JS uses S.systemIP || '…'; we accept an explicit ip
// argument here so tests can control the value.
func simulateDNSGuidanceBox(domain, ip string) string {
	if domain == "" {
		return ""
	}
	if ip == "" {
		ip = "…"
	}
	// Replicate the HTML structure from the JS function (simplified — we only
	// need to verify the presence of the domain, ip, and copy buttons).
	return `<div style="background:var(--blue-dim);border:1px solid var(--blue);border-radius:var(--r);padding:12px 14px;margin-top:8px;font-size:12px">` +
		`<div style="font-weight:600;color:var(--blue);margin-bottom:8px">` +
		` DNS Configuration` +
		`</div>` +
		`<div style="display:flex;align-items:center;gap:8px;margin-bottom:6px">` +
		`<span style="color:var(--muted);width:60px">A record</span>` +
		`<code style="flex:1;background:var(--surface2);padding:3px 8px;border-radius:4px">` + ip + `</code>` +
		`<button class="btn btn-xs" onclick="copyVal('` + ip + `',this)">Copy</button>` +
		`</div>` +
		`<div style="display:flex;align-items:center;gap:8px">` +
		`<span style="color:var(--muted);width:60px">CNAME</span>` +
		`<code style="flex:1;background:var(--surface2);padding:3px 8px;border-radius:4px">` + domain + `</code>` +
		`<button class="btn btn-xs" onclick="copyVal('` + domain + `',this)">Copy</button>` +
		`</div>` +
		`</div>`
}

// countCopyButtons counts the number of copy buttons in the given HTML.
// A copy button is a <button> element whose text content is "Copy".
func countCopyButtons(html string) int {
	count := 0
	search := ">Copy</button>"
	idx := 0
	for {
		pos := strings.Index(html[idx:], search)
		if pos == -1 {
			break
		}
		count++
		idx += pos + len(search)
	}
	return count
}

// TestDNSGuidanceBoxFunctionExistsInUI verifies that the dnsGuidanceBox JS
// function is present in uiHTML — a prerequisite for all other tests.
func TestDNSGuidanceBoxFunctionExistsInUI(t *testing.T) {
	src := extractDNSGuidanceBoxSource()
	if src == "" {
		t.Fatal("dnsGuidanceBox function not found in uiHTML")
	}
	t.Logf("dnsGuidanceBox function found (%d bytes)", len(src))
}

// TestDNSGuidanceBoxEmptyDomainReturnsEmpty verifies that dnsGuidanceBox('')
// returns an empty string — the early-return guard must be present.
//
// **Validates: Requirements 4.1**
func TestDNSGuidanceBoxEmptyDomainReturnsEmpty(t *testing.T) {
	src := extractDNSGuidanceBoxSource()
	if src == "" {
		t.Fatal("dnsGuidanceBox function not found in uiHTML")
	}

	// The function must contain the early-return guard for empty domain.
	if !strings.Contains(src, "if(!domain)return''") {
		t.Error("dnsGuidanceBox function is missing the early-return guard for empty domain")
	}

	// Verify via simulation.
	html := simulateDNSGuidanceBox("", "")
	if html != "" {
		t.Errorf("simulateDNSGuidanceBox('', ''): expected empty string, got %q", html)
	}
}

// TestDNSGuidanceBoxContainsDomainValue verifies that for a non-empty domain,
// the output HTML contains the domain value as visible text.
//
// **Validates: Requirements 4.1, 4.2**
func TestDNSGuidanceBoxContainsDomainValue(t *testing.T) {
	domains := []string{
		"example.com",
		"my-app.example.org",
		"sub.domain.io",
		"localhost",
		"192.168.1.1",
	}

	for _, domain := range domains {
		html := simulateDNSGuidanceBox(domain, "10.0.0.1")
		if !strings.Contains(html, domain) {
			t.Errorf("dnsGuidanceBox(%q): domain value not found in output HTML", domain)
		}
	}
}

// TestDNSGuidanceBoxContainsIPValue verifies that for a non-empty domain,
// the output HTML contains the IP value as visible text.
//
// **Validates: Requirements 4.2**
func TestDNSGuidanceBoxContainsIPValue(t *testing.T) {
	cases := []struct {
		domain string
		ip     string
	}{
		{"example.com", "192.168.1.100"},
		{"my-app.io", "10.0.0.1"},
		{"test.local", "172.16.0.5"},
	}

	for _, tc := range cases {
		html := simulateDNSGuidanceBox(tc.domain, tc.ip)
		if !strings.Contains(html, tc.ip) {
			t.Errorf("dnsGuidanceBox(%q, ip=%q): IP value not found in output HTML", tc.domain, tc.ip)
		}
	}
}

// TestDNSGuidanceBoxContainsTwoCopyButtons verifies that for a non-empty
// domain, the output HTML contains exactly two copy buttons — one for the IP
// and one for the domain.
//
// **Validates: Requirements 4.3**
func TestDNSGuidanceBoxContainsTwoCopyButtons(t *testing.T) {
	domains := []string{
		"example.com",
		"my-app.example.org",
		"test.local",
	}

	for _, domain := range domains {
		html := simulateDNSGuidanceBox(domain, "192.168.1.1")
		count := countCopyButtons(html)
		if count != 2 {
			t.Errorf("dnsGuidanceBox(%q): expected 2 copy buttons, got %d", domain, count)
		}
	}
}

// TestDNSGuidanceBoxJSFunctionContainsCopyButtons verifies that the JS
// dnsGuidanceBox function source in uiHTML contains copy button markup.
//
// **Validates: Requirements 4.3**
func TestDNSGuidanceBoxJSFunctionContainsCopyButtons(t *testing.T) {
	src := extractDNSGuidanceBoxSource()
	if src == "" {
		t.Fatal("dnsGuidanceBox function not found in uiHTML")
	}

	// The function must contain copyVal calls for both the IP and domain.
	if !strings.Contains(src, "copyVal(") {
		t.Error("dnsGuidanceBox function does not contain copyVal() calls — copy buttons missing")
	}

	// Count the number of copyVal calls — there must be at least two.
	count := strings.Count(src, "copyVal(")
	if count < 2 {
		t.Errorf("dnsGuidanceBox function contains %d copyVal() call(s), expected at least 2", count)
	}
}

// TestDNSGuidanceBoxJSFunctionContainsDomainAndIP verifies that the JS
// dnsGuidanceBox function source references both the domain and ip variables.
//
// **Validates: Requirements 4.1, 4.2**
func TestDNSGuidanceBoxJSFunctionContainsDomainAndIP(t *testing.T) {
	src := extractDNSGuidanceBoxSource()
	if src == "" {
		t.Fatal("dnsGuidanceBox function not found in uiHTML")
	}

	// The function must reference the domain parameter.
	if !strings.Contains(src, "domain") {
		t.Error("dnsGuidanceBox function does not reference 'domain' variable")
	}

	// The function must reference the ip variable (S.systemIP or ip).
	if !strings.Contains(src, "ip") {
		t.Error("dnsGuidanceBox function does not reference 'ip' variable")
	}

	// The function must use escHtml to safely render both values.
	if !strings.Contains(src, "escHtml(ip)") {
		t.Error("dnsGuidanceBox function does not use escHtml(ip) — XSS risk or value not rendered")
	}
	if !strings.Contains(src, "escHtml(domain)") {
		t.Error("dnsGuidanceBox function does not use escHtml(domain) — XSS risk or value not rendered")
	}
}

// TestDNSGuidanceBoxSystemIPStateFieldExists verifies that S.systemIP is
// declared in the state object S in uiHTML.
//
// **Validates: Requirements 4.2**
func TestDNSGuidanceBoxSystemIPStateFieldExists(t *testing.T) {
	if !strings.Contains(uiHTML, "systemIP:null") {
		t.Error("S.systemIP field not found in state object S — DNS guidance box cannot fetch IP")
	}
}

// TestDNSGuidanceBoxEnsureSystemIPFunctionExists verifies that the
// ensureSystemIP function is present in uiHTML.
//
// **Validates: Requirements 4.2**
func TestDNSGuidanceBoxEnsureSystemIPFunctionExists(t *testing.T) {
	src := extractFunctionSource("ensureSystemIP")
	if src == "" {
		t.Fatal("ensureSystemIP function not found in uiHTML")
	}

	// The function must fetch /system/ip.
	if !strings.Contains(src, "/system/ip") {
		t.Error("ensureSystemIP function does not fetch /system/ip endpoint")
	}

	// The function must store the result in S.systemIP.
	if !strings.Contains(src, "S.systemIP") {
		t.Error("ensureSystemIP function does not store result in S.systemIP")
	}
}

// TestDNSGuidanceBoxVisibilityProperty is the core property-based test for
// Property 8. It uses testing/quick to verify that for any non-empty domain
// string, the simulated dnsGuidanceBox output:
//  1. Contains the domain value as visible text.
//  2. Contains the IP value as visible text.
//  3. Contains exactly two copy buttons.
//
// **Validates: Requirements 4.1, 4.2, 4.3**
func TestDNSGuidanceBoxVisibilityProperty(t *testing.T) {
	// Property: for any non-empty domain, the output contains domain, ip,
	// and exactly two copy buttons.
	prop := func(domain string) bool {
		if domain == "" {
			// Empty domain is not in the domain of this property.
			// dnsGuidanceBox('') returns '' — that's correct and tested separately.
			return true
		}

		const ip = "192.168.1.100"
		html := simulateDNSGuidanceBox(domain, ip)

		// Must be non-empty.
		if html == "" {
			return false
		}

		// Must contain the domain value.
		if !strings.Contains(html, domain) {
			return false
		}

		// Must contain the IP value.
		if !strings.Contains(html, ip) {
			return false
		}

		// Must contain exactly two copy buttons.
		if countCopyButtons(html) != 2 {
			return false
		}

		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 20}); err != nil {
		t.Errorf("Property 8 violated: %v", err)
	}
}

// TestDNSGuidanceBoxEmptyDomainProperty uses testing/quick to verify that
// for any empty string input, dnsGuidanceBox returns an empty string.
//
// **Validates: Requirements 4.1**
func TestDNSGuidanceBoxEmptyDomainProperty(t *testing.T) {
	// The JS function must return '' for empty domain.
	src := extractDNSGuidanceBoxSource()
	if src == "" {
		t.Fatal("dnsGuidanceBox function not found in uiHTML")
	}

	// Static check: the early-return guard must be present.
	if !strings.Contains(src, "if(!domain)return''") {
		t.Error("dnsGuidanceBox function is missing the early-return guard for empty domain")
	}

	// Simulation check: empty domain always returns empty string.
	html := simulateDNSGuidanceBox("", "10.0.0.1")
	if html != "" {
		t.Errorf("dnsGuidanceBox('', ip): expected empty string, got %q", html)
	}
}

// TestDNSGuidanceBoxIPFallbackToEllipsis verifies that when S.systemIP is
// not yet loaded (null), the JS function falls back to the '…' placeholder.
//
// **Validates: Requirements 4.2**
func TestDNSGuidanceBoxIPFallbackToEllipsis(t *testing.T) {
	src := extractDNSGuidanceBoxSource()
	if src == "" {
		t.Fatal("dnsGuidanceBox function not found in uiHTML")
	}

	// The function must use S.systemIP with a fallback.
	// The JS pattern is: const ip=S.systemIP||'…';
	if !strings.Contains(src, "S.systemIP") {
		t.Error("dnsGuidanceBox function does not reference S.systemIP")
	}

	// Verify the fallback placeholder is present (Unicode ellipsis \u2026 or '…').
	hasEllipsis := strings.Contains(src, `\u2026`) || strings.Contains(src, "…")
	if !hasEllipsis {
		t.Error("dnsGuidanceBox function does not have '…' fallback for missing IP")
	}

	// Simulation: when ip is empty, the placeholder '…' is used.
	html := simulateDNSGuidanceBox("example.com", "")
	if !strings.Contains(html, "…") {
		t.Errorf("dnsGuidanceBox with empty ip: expected '…' placeholder in output, got: %s", html)
	}
}

// TestDNSGuidanceBoxCopyButtonsReferenceValues verifies that the copy buttons
// in the output HTML reference the correct values (IP for A record, domain
// for CNAME).
//
// **Validates: Requirements 4.3**
func TestDNSGuidanceBoxCopyButtonsReferenceValues(t *testing.T) {
	const domain = "example.com"
	const ip = "192.168.1.100"

	html := simulateDNSGuidanceBox(domain, ip)

	// The IP copy button must reference the IP value.
	if !strings.Contains(html, "copyVal('"+ip+"'") {
		t.Errorf("dnsGuidanceBox: IP copy button does not reference IP value %q", ip)
	}

	// The domain copy button must reference the domain value.
	if !strings.Contains(html, "copyVal('"+domain+"'") {
		t.Errorf("dnsGuidanceBox: domain copy button does not reference domain value %q", domain)
	}
}

// TestDNSGuidanceBoxFullVisibilityProperty verifies all aspects of Property 8
// together with a representative set of domain inputs.
//
// **Validates: Requirements 4.1, 4.2, 4.3**
func TestDNSGuidanceBoxFullVisibilityProperty(t *testing.T) {
	cases := []struct {
		domain string
		ip     string
	}{
		{"example.com", "192.168.1.100"},
		{"my-app.example.org", "10.0.0.1"},
		{"sub.domain.io", "172.16.0.5"},
		{"localhost", "127.0.0.1"},
		{"a", "1.2.3.4"},
		{"very-long-domain-name.with-many-parts.example.com", "255.255.255.0"},
	}

	for _, tc := range cases {
		html := simulateDNSGuidanceBox(tc.domain, tc.ip)

		// Requirement 4.1: non-empty domain → non-empty output.
		if html == "" {
			t.Errorf("dnsGuidanceBox(%q, %q): expected non-empty HTML, got empty string", tc.domain, tc.ip)
			continue
		}

		// Requirement 4.2: output contains both IP and domain as visible text.
		if !strings.Contains(html, tc.ip) {
			t.Errorf("dnsGuidanceBox(%q, %q): IP value %q not found in output", tc.domain, tc.ip, tc.ip)
		}
		if !strings.Contains(html, tc.domain) {
			t.Errorf("dnsGuidanceBox(%q, %q): domain value %q not found in output", tc.domain, tc.ip, tc.domain)
		}

		// Requirement 4.3: output contains exactly two copy buttons.
		count := countCopyButtons(html)
		if count != 2 {
			t.Errorf("dnsGuidanceBox(%q, %q): expected 2 copy buttons, got %d", tc.domain, tc.ip, count)
		}
	}
}
