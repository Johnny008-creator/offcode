package webserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Johnny008-creator/offcode/internal/config"
)

func TestLocalRequestBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin, site string
		want                     int
	}{
		{"local client", "127.0.0.1:7878", "", "", 204},
		{"browser", "localhost:7878", "http://localhost:7878", "same-origin", 204},
		{"IPv6", "[::1]:7878", "http://[::1]:7878", "same-origin", 204},
		{"navigation", "127.0.0.1:7878", "", "none", 204},
		{"rebinding", "attacker.example:7878", "", "same-origin", 403},
		{"foreign origin", "127.0.0.1:7878", "https://attacker.example", "", 403},
		{"sandboxed preview", "127.0.0.1:7878", "null", "", 403},
		{"other local service", "localhost:7878", "http://localhost:8888", "", 403},
		{"cross-site image", "127.0.0.1:7878", "", "cross-site", 403},
		{"same-site service", "localhost:7878", "", "same-site", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, method := range []string{"GET", "POST", "OPTIONS"} {
				mux := http.NewServeMux()
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
				s := &Server{mux: mux}
				r := httptest.NewRequest(method, "http://"+tc.host+"/api/message", nil)
				r.Header.Set("Origin", tc.origin)
				r.Header.Set("Sec-Fetch-Site", tc.site)
				w := httptest.NewRecorder()
				s.handler().ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("%s: got %d, want %d", method, w.Code, tc.want)
				}
			}
		})
	}
}

func TestListenRejectsNonLoopback(t *testing.T) {
	for _, addr := range []string{":0", "0.0.0.0:0", "[::]:0", "192.0.2.1:0", "localhost:0"} {
		_, l, err := listen(addr, config.Config{})
		if err == nil {
			l.Close()
			t.Fatalf("accepted %q", addr)
		}
	}
}
