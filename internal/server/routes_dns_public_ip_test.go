package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func resetPrimaryIPStateForTest(t *testing.T) {
	t.Helper()
	oldDetector := primaryIPv4Detector
	oldLookup := dnsLookupIP
	oldValue := primaryIPv4Value
	oldEnv, hadEnv := os.LookupEnv("VESSEL_PUBLIC_IP")

	t.Cleanup(func() {
		primaryIPv4Detector = oldDetector
		dnsLookupIP = oldLookup
		primaryIPv4Once = sync.Once{}
		primaryIPv4Value = oldValue
		if hadEnv {
			_ = os.Setenv("VESSEL_PUBLIC_IP", oldEnv)
		} else {
			_ = os.Unsetenv("VESSEL_PUBLIC_IP")
		}
	})

	primaryIPv4Once = sync.Once{}
	primaryIPv4Value = ""
}

func TestGetPrimaryIPv4UsesEnvOverride(t *testing.T) {
	resetPrimaryIPStateForTest(t)

	if err := os.Setenv("VESSEL_PUBLIC_IP", "13.234.48.241"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	primaryIPv4Detector = detectAdvertisedIPv4

	if got := getPrimaryIPv4(); got != "13.234.48.241" {
		t.Fatalf("getPrimaryIPv4() = %q, want %q", got, "13.234.48.241")
	}
}

func TestDomainDNSStatusRequiresExpectedPublicIPMatch(t *testing.T) {
	resetPrimaryIPStateForTest(t)

	primaryIPv4Detector = func() string { return "13.234.48.241" }
	dnsLookupIP = func(host string) ([]net.IP, error) {
		if host != "umami.example.com" {
			t.Fatalf("LookupIP host = %q, want %q", host, "umami.example.com")
		}
		return []net.IP{net.ParseIP("172.31.39.190")}, nil
	}

	status := domainDNSStatus("umami.example.com")
	if status.Resolved {
		t.Fatal("domainDNSStatus() marked mismatched DNS as resolved")
	}
	if status.MatchesExpected {
		t.Fatal("domainDNSStatus() incorrectly marked mismatched DNS as expected")
	}
	if status.ExpectedIP != "13.234.48.241" {
		t.Fatalf("ExpectedIP = %q, want %q", status.ExpectedIP, "13.234.48.241")
	}
	if !strings.Contains(status.Error, "expected 13.234.48.241") {
		t.Fatalf("Error = %q, want mismatch message with expected IP", status.Error)
	}
}

func TestValidateDomainDNSReadyRequiresExpectedPublicIPMatch(t *testing.T) {
	resetPrimaryIPStateForTest(t)

	primaryIPv4Detector = func() string { return "13.234.48.241" }
	dnsLookupIP = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("13.234.48.241")}, nil
	}
	if err := validateDomainDNSReady("umami.example.com"); err != nil {
		t.Fatalf("validateDomainDNSReady() error = %v, want nil", err)
	}

	dnsLookupIP = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("172.31.39.190")}, nil
	}
	if err := validateDomainDNSReady("umami.example.com"); err == nil {
		t.Fatal("validateDomainDNSReady() succeeded for mismatched DNS")
	}
}

func TestSystemIPEndpointReturnsAdvertisedPublicIP(t *testing.T) {
	resetPrimaryIPStateForTest(t)

	primaryIPv4Detector = func() string { return "13.234.48.241" }
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/system/ip", systemIP())

	req := httptest.NewRequest(http.MethodGet, "/system/ip", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /system/ip status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "13.234.48.241") {
		t.Fatalf("GET /system/ip body = %s, want public IP", w.Body.String())
	}
}
