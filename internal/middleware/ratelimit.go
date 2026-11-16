package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter 按 key（这里是客户端 IP）维护独立令牌桶。
// 后台定期清理长时间不活跃的条目，避免 map 无限增长。
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      float64
	burst    int
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rps,
		burst:    burst,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) get(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	v, ok := rl.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rate.Limit(rl.rps), rl.burst)}
		rl.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for k, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware 超限时直接返回 429，不进入代理逻辑。
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ClientIP(r)
		if !rl.get(key).Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
