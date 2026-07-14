package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNoSlashDoesNotPanic proves finding #2 is fixed: a malformed repo
// fullname (no "/") must return a tool error, not panic the process.
func TestNoSlashDoesNotPanic(t *testing.T) {
	createGitHubClient(context.Background(), "dummy_test_token_not_real")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AddStar panicked on malformed input: %v", r)
		}
	}()

	res, _, err := AddStar(context.Background(), nil, &AddStarParams{
		Fullnames: []string{"noslash", "owner/repo", "", "/"},
	})
	if err != nil {
		t.Fatalf("unexpected Go error (want a tool-level error, not a Go error): %v", err)
	}
	txt := res.Content[0].(*mcp.TextContent).Text
	t.Logf("AddStar result: %s", txt)
}

// TestOriginGuardBlocksHostileOrigin proves finding #1's fix rejects a
// browser request whose Origin is not loopback (DNS-rebinding/CSRF guard).
func TestOriginGuardBlocksHostileOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := requireLocalOrigin(inner)

	// Hostile origin -> must be blocked
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:24240/", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("hostile Origin: want 403, got %d", rec.Code)
	}

	// Loopback origin -> must pass through
	req2 := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:24240/", nil)
	req2.Header.Set("Origin", "http://localhost:5173")
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("localhost Origin: want 200, got %d", rec2.Code)
	}

	// No Origin header (typical non-browser MCP client) -> must pass through
	req3 := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:24240/", nil)
	rec3 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("no Origin: want 200, got %d", rec3.Code)
	}
}
