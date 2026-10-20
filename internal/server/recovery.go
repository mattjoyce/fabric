package restapi

import (
	"io"
	"strings"

	"github.com/gin-gonic/gin"
)

// secretHeaderPrefixes are the request header lines that the recovery dump
// must not print. gin masks Authorization itself but keeps the others, so
// X-API-Key would be written to the log in clear. The names are in the
// canonical form that net/http uses in a request dump.
var secretHeaderPrefixes = []string{
	"X-Api-Key:",
	"Cookie:",
	"Proxy-Authorization:",
}

// redactRequestDump masks the value of each secret header line in text.
func redactRequestDump(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		for _, prefix := range secretHeaderPrefixes {
			if len(line) >= len(prefix) && strings.EqualFold(line[:len(prefix)], prefix) {
				// Keep the "\r" that the request dump puts at each line end.
				masked := line[:len(prefix)] + " *"
				if strings.HasSuffix(line, "\r") {
					masked += "\r"
				}
				lines[i] = masked
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// redactingWriter masks secret request headers in what gin.Recovery writes.
// Recovery prints the whole request dump in one Write call, for both the
// debug-mode panic dump and the broken-pipe dump.
type redactingWriter struct {
	out io.Writer
}

func (w redactingWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(w.out, redactRequestDump(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// recoveryMiddleware is gin.Recovery with the secret headers masked in its
// dumps. It writes to gin.DefaultErrorWriter, like gin.Recovery.
func recoveryMiddleware() gin.HandlerFunc {
	return gin.RecoveryWithWriter(redactingWriter{out: gin.DefaultErrorWriter})
}
