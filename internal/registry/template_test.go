package registry

import "testing"

func TestNewLoadsEmbeddedYAMLTemplates(t *testing.T) {
	reg := New()

	tmpl, ok := reg.Get("nginx-proxy-manager")
	if !ok {
		t.Fatalf("nginx-proxy-manager template was not loaded")
	}
	if tmpl.Name != "Nginx Proxy Manager" {
		t.Fatalf("template name = %q, want Nginx Proxy Manager", tmpl.Name)
	}
}

func TestListIsStable(t *testing.T) {
	reg := New()
	list := reg.List()

	for i := 1; i < len(list); i++ {
		prev, cur := list[i-1], list[i]
		if prev.Category > cur.Category || (prev.Category == cur.Category && prev.Name > cur.Name) {
			t.Fatalf("templates are not sorted at %d: %q/%q before %q/%q", i, prev.Category, prev.Name, cur.Category, cur.Name)
		}
	}
}
