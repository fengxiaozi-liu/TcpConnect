package interfaces

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHTTPRejectsCrossOriginAndNonJSON(t *testing.T) {
	b := &Bridge{}
	h := b.Handler(fstest.MapFS{}, "127.0.0.1:17839")
	for _, tc := range []struct{ origin, content, host string }{{"https://evil.example", "application/json", "127.0.0.1:17839"}, {"", "text/plain", "127.0.0.1:17839"}, {"", "application/json", "evil.example"}} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:17839/api", strings.NewReader(`{"method":"Export"}`))
		r.Host = tc.host
		r.Header.Set("Content-Type", tc.content)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("cross-origin request accepted: %d", w.Code)
		}
	}
}
