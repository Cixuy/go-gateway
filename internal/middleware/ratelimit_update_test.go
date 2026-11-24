package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 热加载后，已存在的访客桶也必须换成新参数：以前只改结构体字段，
// 老访客仍按旧 rps 429，日志却打印“限流已更新”。
func TestRateLimiterUpdateAppliesToExistingVisitor(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := func() int {
		r := httptest.NewRequest("GET", "/x", nil)
		r.RemoteAddr = "10.1.2.3:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	if c := req(); c != http.StatusOK { // 桶容量 1：首个放行
		t.Fatalf("首个请求应放行，实得 %d", c)
	}
	if c := req(); c != http.StatusTooManyRequests {
		t.Fatalf("第二个请求应被限流，实得 %d", c)
	}
	rl.Update(1000, 1000)
	for i := 0; i < 5; i++ {
		if c := req(); c != http.StatusOK {
			t.Fatalf("Update 后第 %d 个请求应放行，实得 %d（老访客的桶没换参数）", i+1, c)
		}
	}
}
