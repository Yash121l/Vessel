package server

import "testing"

func TestValidateDeploymentName(t *testing.T) {
	valid := []string{"n8n", "my-app", "app123"}
	for _, name := range valid {
		if err := validateDeploymentName(name); err != nil {
			t.Fatalf("validateDeploymentName(%q) error = %v", name, err)
		}
	}

	invalid := []string{"", "MyApp", "../app", "app_name", "app--name", "-app", "app-"}
	for _, name := range invalid {
		if err := validateDeploymentName(name); err == nil {
			t.Fatalf("validateDeploymentName(%q) succeeded, want error", name)
		}
	}
}

func TestValidateDomain(t *testing.T) {
	valid := []string{"app.example.com", "n8n.company-ops.io"}
	for _, domain := range valid {
		if err := validateDomain(domain); err != nil {
			t.Fatalf("validateDomain(%q) error = %v", domain, err)
		}
	}

	invalid := []string{"https://app.example.com", "localhost", "bad-.example.com", "bad_domain.example.com"}
	for _, domain := range invalid {
		if err := validateDomain(domain); err == nil {
			t.Fatalf("validateDomain(%q) succeeded, want error", domain)
		}
	}
}

func TestValidateEnv(t *testing.T) {
	if err := validateEnv(map[string]string{"APP_SECRET": "x", "_TOKEN": "y"}); err != nil {
		t.Fatalf("validateEnv() error = %v", err)
	}
	if err := validateEnv(map[string]string{"BAD-KEY": "x"}); err == nil {
		t.Fatal("validateEnv() succeeded for invalid key")
	}
}

func TestValidateUsernameAndRole(t *testing.T) {
	if err := validateUsername("ops.user"); err != nil {
		t.Fatalf("validateUsername() error = %v", err)
	}
	if err := validateUsername("../root"); err == nil {
		t.Fatal("validateUsername() succeeded for traversal-like username")
	}
	if err := validateRole(roleOperator); err != nil {
		t.Fatalf("validateRole() error = %v", err)
	}
	if err := validateRole("superuser"); err == nil {
		t.Fatal("validateRole() succeeded for unknown role")
	}
}

func TestValidateSiteFileNameRejectsTraversal(t *testing.T) {
	if err := validateSiteFileName("app.conf"); err != nil {
		t.Fatalf("validateSiteFileName() error = %v", err)
	}
	if err := validateSiteFileName("../nginx.conf"); err == nil {
		t.Fatal("validateSiteFileName() succeeded for traversal")
	}
}
