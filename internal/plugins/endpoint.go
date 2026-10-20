package plugins

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/danielmiessler/fabric/internal/i18n"
)

// RequireSecureBaseURL checks the base URL of a vendor that gets an API key
// or token. It accepts https, and http only for a loopback host, so that
// credentials and prompts do not cross a network in plaintext. An empty URL
// is accepted: the vendor then uses its default.
func RequireSecureBaseURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return errors.New(i18n.T("plugins_error_invalid_base_url"))
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		if isLoopbackHost(parsed.Hostname()) {
			return nil
		}
	}
	return fmt.Errorf(i18n.T("plugins_error_insecure_base_url"), parsed.Scheme+"://"+parsed.Host)
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
