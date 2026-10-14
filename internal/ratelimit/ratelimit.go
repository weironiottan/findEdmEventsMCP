// Package ratelimit provides per-client-IP HTTP rate limiting.
//
// Limits are held in memory, so each server instance enforces them
// independently.
package ratelimit

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// idleTTL is how long a client's limiter is kept after its last request.
const idleTTL = 10 * time.Minute

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter allows each client IP perSecond requests per second, with bursts
// of up to burst requests.
type Limiter struct {
	perSecond rate.Limit
	burst     int

	mu        sync.Mutex
	clients   map[string]*client
	lastSweep time.Time
}

func New(perSecond float64, burst int) *Limiter {
	return &Limiter{
		perSecond: rate.Limit(perSecond),
		burst:     burst,
		clients:   map[string]*client{},
		lastSweep: time.Now(),
	}
}

// Middleware rejects requests over the limit with 429 Too Many Requests.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r), time.Now()) {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(1/float64(l.perSecond)))))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *Limiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastSweep) > idleTTL {
		for k, c := range l.clients {
			if now.Sub(c.lastSeen) > idleTTL {
				delete(l.clients, k)
			}
		}
		l.lastSweep = now
	}

	c, ok := l.clients[ip]
	if !ok {
		c = &client{limiter: rate.NewLimiter(l.perSecond, l.burst)}
		l.clients[ip] = c
	}
	c.lastSeen = now
	return c.limiter.AllowN(now, 1)
}

// clientIP returns the caller's IP. Cloud Run's front end appends the real
// client IP to X-Forwarded-For, so the last entry is trusted; earlier entries
// are client-supplied and may be spoofed.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
