package deploy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

const defaultPublicIPEndpoint = "https://api.ipify.org"

var (
	publicIPEndpoint   = defaultPublicIPEndpoint
	publicIPHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

func ResolvePublicIP(ctx context.Context) (string, error) {
	return resolvePublicIP(ctx, publicIPHTTPClient, publicIPEndpoint)
}

func resolvePublicIP(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve current public IP: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", fmt.Errorf("read current public IP response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resolve current public IP: unexpected status %d", resp.StatusCode)
	}

	value := strings.TrimSpace(string(body))
	if _, err := netip.ParseAddr(value); err != nil {
		return "", fmt.Errorf("resolve current public IP: service returned invalid IP %q", value)
	}

	return value, nil
}
