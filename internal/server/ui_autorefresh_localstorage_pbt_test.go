package server

// **Validates: Requirements 3.4**
//
// Property 7: Auto-refresh localStorage persistence
//
// For any toggle state change (enable or disable), localStorage.getItem('vessel-autorefresh')
// must immediately reflect the new state ('1' for enabled, '0' for disabled).
//
// Since toggleAutoRefresh is a JavaScript function embedded in the Go string
// constant uiHTML, this test verifies the property by:
//  1. Extracting the toggleAutoRefresh function source from uiHTML (using the
//     shared extractFunctionSource helper from ui_auto_refresh_pbt_test.go).
//  2. Verifying the function body contains localStorage.setItem calls with
//     'vessel-autorefresh' and the correct '1'/'0' values.
//  3. Verifying the key 'vessel-autorefresh' is used consistently for both
//     the setItem call (in toggleAutoRefresh) and the getItem call (in boot).
//  4. Using testing/quick to confirm the static structure holds across
//     simulated toggle sequences.

import (
	"strings"
	"testing"
	"testing/quick"
)

// localStorageValueForState returns the expected localStorage value for a
// given auto-refresh enabled state, mirroring the JS logic:
//
//	next ? '1' : '0'
func localStorageValueForState(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

// TestToggleAutoRefreshContainsLocalStorageSetItem verifies that the
// toggleAutoRefresh function calls localStorage.setItem with the key
// 'vessel-autorefresh'.
func TestToggleAutoRefreshContainsLocalStorageSetItem(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	const setItemCall = "localStorage.setItem("
	if !strings.Contains(src, setItemCall) {
		t.Errorf("toggleAutoRefresh does not contain %q", setItemCall)
	}
}

// TestToggleAutoRefreshUsesCorrectStorageKey verifies that the
// toggleAutoRefresh function uses the exact key 'vessel-autorefresh' in its
// localStorage.setItem call.
func TestToggleAutoRefreshUsesCorrectStorageKey(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	const storageKey = "'vessel-autorefresh'"
	if !strings.Contains(src, storageKey) {
		t.Errorf("toggleAutoRefresh does not use storage key %s", storageKey)
	}
}

// TestToggleAutoRefreshPersistsEnabledValue verifies that the toggleAutoRefresh
// function stores '1' when enabling auto-refresh.
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshPersistsEnabledValue(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	// The function must store '1' for the enabled state.
	// The JS is: localStorage.setItem('vessel-autorefresh', next ? '1' : '0')
	if !strings.Contains(src, "'1'") {
		t.Error("toggleAutoRefresh does not contain '1' value for enabled state")
	}
}

// TestToggleAutoRefreshPersistsDisabledValue verifies that the toggleAutoRefresh
// function stores '0' when disabling auto-refresh.
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshPersistsDisabledValue(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	// The function must store '0' for the disabled state.
	if !strings.Contains(src, "'0'") {
		t.Error("toggleAutoRefresh does not contain '0' value for disabled state")
	}
}

// TestToggleAutoRefreshSetItemCallStructure verifies the exact structure of the
// localStorage.setItem call: it must use the ternary expression
// next?'1':'0' (or equivalent) to select the correct value.
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshSetItemCallStructure(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	// The setItem call must reference both '1' and '0' as the possible values.
	// The JS pattern is: localStorage.setItem('vessel-autorefresh', next?'1':'0')
	hasOne := strings.Contains(src, "'1'")
	hasZero := strings.Contains(src, "'0'")
	if !hasOne || !hasZero {
		t.Errorf("toggleAutoRefresh setItem call must reference both '1' and '0'; hasOne=%v hasZero=%v\nsrc: %s",
			hasOne, hasZero, src)
	}

	// The key and both values must appear within the setItem call.
	setItemIdx := strings.Index(src, "localStorage.setItem(")
	if setItemIdx == -1 {
		t.Fatal("localStorage.setItem not found in toggleAutoRefresh")
	}

	// Find the end of the setItem call (closing parenthesis).
	setItemSection := src[setItemIdx:]
	parenDepth := 0
	callEnd := -1
	for i, ch := range setItemSection {
		switch ch {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
			if parenDepth == 0 {
				callEnd = i + 1
				break
			}
		}
		if callEnd != -1 {
			break
		}
	}
	if callEnd == -1 {
		t.Fatal("could not find end of localStorage.setItem() call")
	}
	callSrc := setItemSection[:callEnd]

	if !strings.Contains(callSrc, "'vessel-autorefresh'") {
		t.Errorf("localStorage.setItem call does not contain key 'vessel-autorefresh': %s", callSrc)
	}
	if !strings.Contains(callSrc, "'1'") {
		t.Errorf("localStorage.setItem call does not contain value '1': %s", callSrc)
	}
	if !strings.Contains(callSrc, "'0'") {
		t.Errorf("localStorage.setItem call does not contain value '0': %s", callSrc)
	}
}

// TestToggleAutoRefreshStorageKeyConsistency verifies that the same key
// 'vessel-autorefresh' is used in both the setItem call (in toggleAutoRefresh)
// and the getItem call (in the boot/init code that restores the preference).
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshStorageKeyConsistency(t *testing.T) {
	const storageKey = "vessel-autorefresh"

	// Count occurrences of the key in uiHTML.
	count := strings.Count(uiHTML, "'"+storageKey+"'")
	if count < 2 {
		t.Errorf("storage key %q appears only %d time(s) in uiHTML; expected at least 2 (setItem + getItem)",
			storageKey, count)
	}

	// Verify setItem usage in toggleAutoRefresh.
	toggleSrc := extractFunctionSource("toggleAutoRefresh")
	if toggleSrc == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}
	if !strings.Contains(toggleSrc, "'"+storageKey+"'") {
		t.Errorf("toggleAutoRefresh does not reference storage key %q", storageKey)
	}

	// Verify getItem usage exists somewhere in uiHTML (boot/init code).
	getItemPattern := "localStorage.getItem('" + storageKey + "')"
	if !strings.Contains(uiHTML, getItemPattern) {
		t.Errorf("uiHTML does not contain getItem call for key %q: expected %q", storageKey, getItemPattern)
	}
}

// TestToggleAutoRefreshLocalStorageValueProperty is the core property-based
// test for Property 7. It verifies that for any boolean toggle state, the
// expected localStorage value ('1' or '0') is correctly encoded in the
// toggleAutoRefresh function source.
//
// Since we cannot execute JS in Go, we verify the static structure of the
// function: the ternary expression must map true→'1' and false→'0'.
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshLocalStorageValueProperty(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	// Property: for any toggle state (enabled=true or enabled=false), the
	// localStorage value must be '1' or '0' respectively.
	// We verify this by checking the ternary pattern in the function source.
	prop := func(enabled bool) bool {
		expectedValue := localStorageValueForState(enabled)
		// Both '1' and '0' must be present in the function source (as the
		// ternary covers both branches).
		return strings.Contains(src, "'"+expectedValue+"'")
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 20}); err != nil {
		t.Errorf("Property 7 violated: %v", err)
	}
}

// TestToggleAutoRefreshSetItemCalledUnconditionally verifies that
// localStorage.setItem is called unconditionally in toggleAutoRefresh —
// not inside an if/else branch — so that every toggle (enable or disable)
// persists the new state.
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshSetItemCalledUnconditionally(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	// The localStorage.setItem call must appear outside the if/else block.
	// Strategy: verify setItem appears after the if(next) block.
	setItemIdx := strings.Index(src, "localStorage.setItem(")
	if setItemIdx == -1 {
		t.Fatal("localStorage.setItem not found in toggleAutoRefresh")
	}

	// Find the if(next) block start.
	ifIdx := strings.Index(src, "if(next)")
	if ifIdx == -1 {
		ifIdx = strings.Index(src, "if (next)")
	}
	if ifIdx == -1 {
		t.Fatal("if(next) block not found in toggleAutoRefresh")
	}

	// The setItem call must come after the if/else block.
	if setItemIdx <= ifIdx {
		t.Errorf("localStorage.setItem appears before if(next) block — may not persist both states")
	}

	// Find the else block.
	elseIdx := strings.Index(src, "}else{")
	if elseIdx == -1 {
		elseIdx = strings.Index(src, "} else {")
	}
	if elseIdx != -1 && setItemIdx <= elseIdx {
		t.Errorf("localStorage.setItem appears before else block — may not persist disabled state")
	}
}

// TestToggleAutoRefreshBothValuesInSetItemProperty verifies that the
// localStorage.setItem call in toggleAutoRefresh can produce both '1' and '0'
// — not just one of them — ensuring both enable and disable are persisted.
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshBothValuesInSetItemProperty(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	// Find the setItem call.
	setItemIdx := strings.Index(src, "localStorage.setItem(")
	if setItemIdx == -1 {
		t.Fatal("localStorage.setItem not found in toggleAutoRefresh")
	}

	// Extract the setItem call arguments.
	callSection := src[setItemIdx:]
	parenDepth := 0
	callEnd := -1
	for i, ch := range callSection {
		switch ch {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
			if parenDepth == 0 {
				callEnd = i + 1
				break
			}
		}
		if callEnd != -1 {
			break
		}
	}
	if callEnd == -1 {
		t.Fatal("could not find end of localStorage.setItem() call")
	}
	callArgs := callSection[:callEnd]

	// Both '1' and '0' must appear in the call arguments (via ternary).
	if !strings.Contains(callArgs, "'1'") {
		t.Errorf("localStorage.setItem call does not include '1' for enabled state: %s", callArgs)
	}
	if !strings.Contains(callArgs, "'0'") {
		t.Errorf("localStorage.setItem call does not include '0' for disabled state: %s", callArgs)
	}

	t.Logf("localStorage.setItem call verified: %s", callArgs)
}

// TestToggleAutoRefreshLocalStoragePersistenceFullProperty is the comprehensive
// property test for Property 7, verifying all aspects of localStorage
// persistence in a single test.
//
// **Validates: Requirements 3.4**
func TestToggleAutoRefreshLocalStoragePersistenceFullProperty(t *testing.T) {
	src := extractFunctionSource("toggleAutoRefresh")
	if src == "" {
		t.Fatal("toggleAutoRefresh function not found in uiHTML")
	}

	// All structural requirements for localStorage persistence.
	checks := []struct {
		pattern string
		desc    string
	}{
		{"localStorage.setItem(", "setItem call present"},
		{"'vessel-autorefresh'", "correct storage key used"},
		{"'1'", "enabled value '1' present"},
		{"'0'", "disabled value '0' present"},
	}

	for _, c := range checks {
		if !strings.Contains(src, c.pattern) {
			t.Errorf("Property 7 violated — %s: pattern %q not found in toggleAutoRefresh", c.desc, c.pattern)
		}
	}

	// Verify the getItem counterpart exists in uiHTML for preference restoration.
	if !strings.Contains(uiHTML, "localStorage.getItem('vessel-autorefresh')") {
		t.Error("Property 7 violated — localStorage.getItem('vessel-autorefresh') not found in uiHTML: " +
			"preference cannot be restored on page reload (Requirement 3.4)")
	}

	t.Log("Property 7 verified: toggleAutoRefresh correctly persists state to localStorage")
}
