package middleware

import (
	"net"
	"net/http"
	"strings"
)

// Chain 按顺序组合中间件，最外层先执行。
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// ClientIP 从请求里取客户端 IP。
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		if i := strings.Index(ip, ","); i > 0 {
			return strings.TrimSpace(ip[:i])
		}
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// StatusWriter 记录响应状态码，供熔断与指标使用。
type StatusWriter struct {
	http.ResponseWriter
	Status int
}

func NewStatusWriter(w http.ResponseWriter) *StatusWriter {
	return &StatusWriter{ResponseWriter: w, Status: http.StatusOK}
}

func (w *StatusWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}
