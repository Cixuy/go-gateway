package middleware

import (
	"net/http"
	"sync"
	"time"
)

type breakerState int

const (
	stateClosed breakerState = iota
	stateOpen
	stateHalfOpen
)

// Breaker 是每条路由一个的熔断器：连续失败到阈值后打开，
// 打开期间直接拒绝，到期后半开放行一个请求探测。
type Breaker struct {
	mu        sync.Mutex
	state     breakerState
	failures  int
	openUntil time.Time

	threshold int
	openFor   time.Duration
}

func NewBreaker(threshold int, openFor time.Duration) *Breaker {
	return &Breaker{threshold: threshold, openFor: openFor}
}

func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case stateOpen:
		if time.Now().After(b.openUntil) {
			b.state = stateHalfOpen
			return true
		}
		return false
	case stateHalfOpen:
		// 半开状态只放一个请求，其余继续拒绝
		return false
	default:
		return true
	}
}

func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = stateClosed
}

func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.state == stateHalfOpen || b.failures >= b.threshold {
		b.state = stateOpen
		b.openUntil = time.Now().Add(b.openFor)
		b.failures = 0
	}
}

func (b *Breaker) State() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// Guard 包装一次后端调用：熔断打开时直接 503；
// 放行后按响应状态记账，5xx 计为失败，其余计为成功。
func (b *Breaker) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !b.Allow() {
			http.Error(w, `{"error":"upstream unavailable (circuit open)"}`, http.StatusServiceUnavailable)
			return
		}
		sw := NewStatusWriter(w)
		next.ServeHTTP(sw, r)
		if sw.Status >= 500 {
			b.Failure()
		} else {
			b.Success()
		}
	})
}
