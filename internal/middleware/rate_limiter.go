package middleware

import (
    "crypto/sha256"
    "encoding/hex"
    "net/http"
    "strings"
    "sync"
    "time"

    "yourapp/internal/auth"
)

// Simple token-bucket limiter per key (user or rotated IP hash)
type bucket struct {
    tokens     float64
    lastRefill time.Time
}

type limiter struct {
    mu       sync.Mutex
    buckets  map[string]*bucket
    rate     float64       // tokens per second
    capacity float64       // burst capacity
    ttl      time.Duration // eviction TTL
}

func newLimiter(rps float64, burst int, ttl time.Duration) *limiter {
    return &limiter{
        buckets:  make(map[string]*bucket),
        rate:     rps,
        capacity: float64(burst),
        ttl:      ttl,
    }
}

func (l *limiter) allow(key string) bool {
    now := time.Now()
    l.mu.Lock()
    defer l.mu.Unlock()
    b, ok := l.buckets[key]
    if !ok {
        b = &bucket{tokens: l.capacity, lastRefill: now}
        l.buckets[key] = b
    }
    // Refill
    elapsed := now.Sub(b.lastRefill).Seconds()
    b.tokens = min(l.capacity, b.tokens+elapsed*l.rate)
    b.lastRefill = now
    if b.tokens >= 1.0 {
        b.tokens -= 1.0
        return true
    }
    return false
}

func min(a, b float64) float64 { if a < b { return a }; return b }

// Rotating IP hasher (daily rotation) to avoid storing raw IPs.
var (
    rotMu    sync.RWMutex
    rotSalt  []byte
    rotDay   int
)

func rotateSaltIfNeeded() {
    d := time.Now().YearDay()
    rotMu.Lock()
    defer rotMu.Unlock()
    if d != rotDay || rotSalt == nil {
        rotDay = d
        // Use time-based bytes; crypto randomness is optional here
        s := sha256.Sum256([]byte(time.Now().Format(time.RFC3339Nano)))
        rotSalt = s[:]
    }
}

func ipKey(r *http.Request) string {
    host := r.RemoteAddr
    // Remove port if present
    if i := strings.LastIndex(host, ":"); i > -1 {
        host = host[:i]
    }
    // If behind proxy, you may parse X-Forwarded-For here (carefully).
    rotateSaltIfNeeded()
    rotMu.RLock()
    salt := make([]byte, len(rotSalt))
    copy(salt, rotSalt)
    rotMu.RUnlock()
    h := sha256.New()
    h.Write(salt)
    h.Write([]byte(host))
    sum := h.Sum(nil)
    // truncate for readability
    return hex.EncodeToString(sum[:8])
}

// RateLimitWith returns middleware limiting requests per principal with config.
// rpm: requests per minute; burst: bucket size; ttl controls bucket eviction (not yet used for cleanup).
func RateLimitWith(rpm int, burst int, ttl time.Duration) func(http.Handler) http.Handler {
    rps := float64(rpm) / 60.0
    if rps <= 0 {
        rps = 0.000001
    }
    if burst <= 0 {
        burst = 1
    }
    lim := newLimiter(rps, burst, ttl)
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := ""
            if sess, ok := auth.SessionFromContext(r.Context()); ok && sess != nil {
                key = "u:" + sess.UserID.String()
            } else {
                key = "ip:" + ipKey(r)
            }
            if ok := lim.allow(key); !ok {
                w.Header().Set("Retry-After", "1")
                http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
