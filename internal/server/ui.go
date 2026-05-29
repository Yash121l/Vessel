package server

import (
	_ "embed"
)

// uiHTML is kept as a package variable so the existing UI-focused tests can
// continue to inspect the rendered source, even though the SPA now lives in an
// embedded asset file instead of a raw Go string literal.
//
//go:embed ui/index.html
var uiHTML string
