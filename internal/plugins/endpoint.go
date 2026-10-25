package plugins

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/danielmiessler/fabric/internal/i18n"
	"github.com/danielmiessler/fabric/internal/util"
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
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New(i18n.T("plugin_invalid_base_url"))
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if util.IsLoopbackHost(parsed.Hostname()) {
			return nil
		}
		// Operator opt-in: FABRIC_ALLOW_INSECURE_LAN lists the private hosts
		// that may use plain http. Unset matches nothing.
		if util.HostAllowedForInsecureHTTP(parsed.Hostname(), os.Getenv("FABRIC_ALLOW_INSECURE_LAN")) {
			return nil
		}
	}
	return fmt.Errorf(i18n.T("plugin_insecure_base_url"), parsed.Scheme+"://"+parsed.Host)
}
