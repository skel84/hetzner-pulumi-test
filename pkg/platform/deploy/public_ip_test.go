package deploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolvePublicIP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10\n"))
	}))
	defer server.Close()

	got, err := resolvePublicIP(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("resolvePublicIP() error = %v", err)
	}
	if got != "203.0.113.10" {
		t.Fatalf("resolvePublicIP() = %q, want 203.0.113.10", got)
	}
}

func TestResolvePublicIPUsesDefaultResolverSettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("203.0.113.11"))
	}))
	defer server.Close()

	originalEndpoint := publicIPEndpoint
	originalClient := publicIPHTTPClient
	publicIPEndpoint = server.URL
	publicIPHTTPClient = server.Client()
	defer func() {
		publicIPEndpoint = originalEndpoint
		publicIPHTTPClient = originalClient
	}()

	got, err := ResolvePublicIP(context.Background())
	if err != nil {
		t.Fatalf("ResolvePublicIP() error = %v", err)
	}
	if got != "203.0.113.11" {
		t.Fatalf("ResolvePublicIP() = %q, want 203.0.113.11", got)
	}
}

func TestResolvePublicIPRejectsBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := resolvePublicIP(context.Background(), server.Client(), server.URL)
	if err == nil {
		t.Fatal("resolvePublicIP() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unexpected status 502") {
		t.Fatalf("resolvePublicIP() error = %q, want status context", err)
	}
}

func TestResolvePublicIPRejectsInvalidIP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-an-ip"))
	}))
	defer server.Close()

	_, err := resolvePublicIP(context.Background(), server.Client(), server.URL)
	if err == nil {
		t.Fatal("resolvePublicIP() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid IP") {
		t.Fatalf("resolvePublicIP() error = %q, want invalid IP context", err)
	}
}
