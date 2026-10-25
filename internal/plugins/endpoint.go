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
		// Opt-in escape hatch for a private model server: FABRIC_ALLOW_INSECURE_LAN
		// is an operator-supplied allowlist of destinations for which plaintext
		// http is accepted, e.g. "192.168.20.0/24", "192.168.20.x", "*.ts.net",
		// or the keyword "lan" for any private/LAN/tailnet host. The intended
		// use is a model server whose hop is already protected by a private
		// network or WireGuard that this URL-only check cannot see. Unset or
		// empty matches nothing, so secure-by-default holds, and only the named
		// hosts are relaxed, not remote http in general.
		if util.HostAllowedForInsecureHTTP(parsed.Hostname(), os.Getenv("FABRIC_ALLOW_INSECURE_LAN")) {
			return nil
		}
	}
	return fmt.Errorf(i18n.T("plugin_insecure_base_url"), parsed.Scheme+"://"+parsed.Host)
}
