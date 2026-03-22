package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"upwind-cli/internal/config"
)

func TestAuthorizationHeaderReportsNonJSONOAuthErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}

		writer.Header().Set("Content-Type", "text/html")
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("<html>upstream auth failure</html>"))
	}))
	defer server.Close()

	provider := NewProvider(server.Client(), config.Runtime{
		AuthURL:      server.URL,
		Audience:     "https://api.upwind.io",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})

	_, err := provider.AuthorizationHeader(context.Background())
	if err == nil {
		t.Fatal("expected AuthorizationHeader to fail")
	}

	if !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("expected HTTP status in error, got %q", err)
	}
	if !strings.Contains(err.Error(), "upstream auth failure") {
		t.Fatalf("expected response body in error, got %q", err)
	}
	if strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("expected HTTP error, got JSON parse error %q", err)
	}
}
