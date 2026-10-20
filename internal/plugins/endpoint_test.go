package plugins

import (
	"strings"
	"testing"
)

func TestRequireSecureBaseURL(t *testing.T) {
	ok := []string{
		"",
		"https://api.example.com/v1",
		"HTTPS://api.example.com",
		"http://localhost:1234/v1",
		"http://127.0.0.1:11434",
		"http://127.1.2.3",
		"http://[::1]:8080",
		"http://app.localhost:3000",
	}
	for _, raw := range ok {
		if err := RequireSecureBaseURL(raw); err != nil {
			t.Errorf("RequireSecureBaseURL(%q) = %v, want nil", raw, err)
		}
	}

	bad := []string{
		"http://api.example.com/v1",
		"http://192.168.1.10:8000/v1",
		"http://localhost.evil.example/v1",
		"ftp://localhost/v1",
		"api.example.com/v1",
		"http://",
	}
	for _, raw := range bad {
		if err := RequireSecureBaseURL(raw); err == nil {
			t.Errorf("RequireSecureBaseURL(%q) = nil, want an error", raw)
		}
	}
}

func TestRequireSecureBaseURLErrorOmitsCredentials(t *testing.T) {
	err := RequireSecureBaseURL("http://user:secret@api.example.com/v1?key=abc")
	if err == nil {
		t.Fatal("want an error for plaintext http to a remote host")
	}
	for _, leak := range []string{"secret", "user", "key=abc"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error %q leaks %q", err, leak)
		}
	}
}
