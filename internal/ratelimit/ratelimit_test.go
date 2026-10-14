package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowPerClient(t *testing.T) {
	l := New(1, 2)
	now := time.Now()

	for i, want := range []bool{true, true, false} {
		if got := l.allow("1.1.1.1", now); got != want {
			t.Errorf("request %d: allow = %v, want %v", i, got, want)
		}
	}
	if !l.allow("2.2.2.2", now) {
		t.Error("other client should have its own budget")
	}
	if !l.allow("1.1.1.1", now.Add(time.Second)) {
		t.Error("budget should refill after a second")
	}
}

func TestSweepIdleClients(t *testing.T) {
	l := New(1, 1)
	now := time.Now()
	l.allow("1.1.1.1", now)
	l.allow("2.2.2.2", now.Add(2*idleTTL))
	if _, ok := l.clients["1.1.1.1"]; ok {
		t.Error("idle client was not swept")
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		xff, remote, want string
	}{
		{"", "3.3.3.3:1234", "3.3.3.3"},
		{"4.4.4.4", "10.0.0.1:1", "4.4.4.4"},
		{"6.6.6.6, 5.5.5.5", "10.0.0.1:1", "5.5.5.5"},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = tt.remote
		if tt.xff != "" {
			r.Header.Set("X-Forwarded-For", tt.xff)
		}
		if got := clientIP(r); got != tt.want {
			t.Errorf("clientIP(xff=%q, remote=%q) = %q, want %q", tt.xff, tt.remote, got, tt.want)
		}
	}
}

func TestMiddleware(t *testing.T) {
	h := New(1, 1).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
		if w.Code != want {
			t.Errorf("request %d: status = %d, want %d", i, w.Code, want)
		}
	}
}
