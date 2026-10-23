package restapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielmiessler/fabric/internal/i18n"
	"github.com/gin-gonic/gin"
)

const APIKeyHeader = "X-API-Key"

// minAPIKeyLength is the key length below which the server warns. A short
// key is easy to guess even with the limit on wrong keys.
const minAPIKeyLength = 16

// A client that sends maxAuthFailures wrong keys within authFailureWindow
// gets 429 for authLockout, for every request, also one with the right key.
// The limit lets a person mistype a key a few times, and it keeps a script
// from trying keys at network speed.
const (
	maxAuthFailures   = 10
	authFailureWindow = time.Minute
	authLockout       = time.Minute
	// maxTrackedClients bounds the memory that the limiter uses.
	maxTrackedClients = 10000
)

type authFailures struct {
	count       int
	windowStart time.Time
	lockedUntil time.Time
}

// authFailureLimiter counts wrong keys per client address.
type authFailureLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	clients map[string]*authFailures
}

func newAuthFailureLimiter() *authFailureLimiter {
	return &authFailureLimiter{now: time.Now, clients: make(map[string]*authFailures)}
}

// retryAfter returns how long a locked client must wait, or 0 when it may try.
func (l *authFailureLimiter) retryAfter(client string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry, ok := l.clients[client]; ok {
		if wait := entry.lockedUntil.Sub(l.now()); wait > 0 {
			return wait
		}
	}
	return 0
}

// fail records a wrong key and returns how long the client is locked, or 0.
func (l *authFailureLimiter) fail(client string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, ok := l.clients[client]
	if !ok {
		if len(l.clients) >= maxTrackedClients {
			l.prune(now)
		}
		entry = &authFailures{windowStart: now}
		l.clients[client] = entry
	}
	if now.Sub(entry.windowStart) > authFailureWindow {
		entry.count, entry.windowStart = 0, now
	}
	entry.count++
	if entry.count >= maxAuthFailures {
		entry.count, entry.windowStart = 0, now
		entry.lockedUntil = now.Add(authLockout)
		return authLockout
	}
	return 0
}

// prune drops each client that is not locked, so that a flood of addresses
// cannot grow the table. Locked clients stay locked.
func (l *authFailureLimiter) prune(now time.Time) {
	for client, entry := range l.clients {
		if !entry.lockedUntil.After(now) {
			delete(l.clients, client)
		}
	}
}

// requireAPIKeyForBind rejects a non-loopback bind address that has no
// API key. An empty or unspecified host binds each interface, and that
// counts as non-loopback. It warns when the key is short.
func requireAPIKeyForBind(address, apiKey string) error {
	if apiKey != "" && len(apiKey) < minAPIKeyLength {
		slog.Warn("API key is short: use a random key of 16 or more characters", "length", len(apiKey))
	}
	// Only the exact name "localhost" or a loopback IP is accepted. A name
	// such as "app.localhost" goes to DNS on some systems, and the server
	// can then listen on a public interface with no key.
	host := hostOnly(address)
	if apiKey != "" || host == "localhost" || net.ParseIP(host).IsLoopback() {
		return nil
	}
	return fmt.Errorf(i18n.T("server_api_key_required"), address)
}

// APIKeyMiddleware validates API key for protected endpoints.
// Swagger documentation endpoints (/swagger/*) are exempt from authentication
// to allow users to browse and test the API documentation freely.
func APIKeyMiddleware(apiKey string) gin.HandlerFunc {
	return apiKeyMiddleware(apiKey, newAuthFailureLimiter())
}

func apiKeyMiddleware(apiKey string, limiter *authFailureLimiter) gin.HandlerFunc {
	// Hash the configured key so the comparison avoids leaking its length through timing.
	expectedKey := sha256.Sum256([]byte(apiKey))
	return func(c *gin.Context) {
		// Skip authentication for Swagger documentation endpoints
		// This allows public access to API docs even when authentication is enabled
		if strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			c.Next()
			return
		}

		client := c.ClientIP()
		if wait := limiter.retryAfter(client); wait > 0 {
			c.Header("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many wrong API keys"})
			return
		}

		headerApiKey := c.GetHeader(APIKeyHeader)

		if headerApiKey == "" {
			slog.Warn("API key missing", "client", c.ClientIP(), "path", c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing API Key"})
			return
		}

		headerKey := sha256.Sum256([]byte(headerApiKey))
		if subtle.ConstantTimeCompare(headerKey[:], expectedKey[:]) != 1 {
			// An operator can find repeated guesses in this log.
			slog.Warn("API key wrong", "client", client, "path", c.Request.URL.Path)
			if wait := limiter.fail(client); wait > 0 {
				slog.Warn("too many wrong API keys: client locked out", "client", client, "for", wait)
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Wrong API Key"})
			return
		}

		c.Next()
	}
}

// auditChange writes a structured log event after a request that changes a
// stored object or the configuration. The event has no body and no
// credentials. With the auth failure events above, it is the audit log.
func auditChange(c *gin.Context) {
	c.Next()
	slog.Info("audit: change", "method", c.Request.Method, "path", c.Request.URL.Path,
		"status", c.Writer.Status(), "client", c.ClientIP())
}
