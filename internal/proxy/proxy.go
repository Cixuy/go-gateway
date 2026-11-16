package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/Cixuy/go-gateway/internal/config"
	"github.com/Cixuy/go-gateway/internal/route"
)

// New 为一条路由构造反向代理。
// 每次请求重新选后端，才能配合权重做分流。
func New(rc config.RouteConfig) (http.Handler, error) {
	targets := make(map[string]*httputil.ReverseProxy, len(rc.Backends))
	for _, b := range rc.Backends {
		u, err := url.Parse(b.URL)
		if err != nil {
			return nil, err
		}
		targets[b.URL] = &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = u.Scheme
				req.URL.Host = u.Host
				req.URL.Path = route.RewritePath(rc.Prefix, req.URL.Path)
				req.Host = u.Host
				// 让后端知道原始路径前缀，便于拼接跳转地址
				req.Header.Set("X-Forwarded-Prefix", rc.Prefix)
				if req.Header.Get("X-Forwarded-Proto") == "" {
					proto := "http"
					if req.TLS != nil {
						proto = "https"
					}
					req.Header.Set("X-Forwarded-Proto", proto)
				}
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				msg := err.Error()
				if len(msg) > 200 {
					msg = msg[:200]
				}
				w.Write([]byte(`{"error":"upstream error","detail":"` + strings.ReplaceAll(msg, `"`, `'`) + `"}`))
			},
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend := route.PickBackend(rc.Backends)
		p, ok := targets[backend.URL]
		if !ok {
			http.Error(w, `{"error":"no backend"}`, http.StatusBadGateway)
			return
		}
		p.ServeHTTP(w, r)
	}), nil
}
