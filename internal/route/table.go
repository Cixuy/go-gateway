package route

import (
	"math/rand"
	"strings"

	"github.com/Cixuy/go-gateway/internal/config"
)

// Match 按前缀最长匹配，返回命中的路由。
func Match(routes []config.RouteConfig, path string) *config.RouteConfig {
	var best *config.RouteConfig
	for i := range routes {
		r := &routes[i]
		if strings.HasPrefix(path, r.Prefix) {
			if best == nil || len(r.Prefix) > len(best.Prefix) {
				best = r
			}
		}
	}
	return best
}

// PickBackend 按权重挑一个后端。权重相同的多个实例即轮询的近似实现。
func PickBackend(backends []config.BackendConfig) config.BackendConfig {
	total := 0
	for _, b := range backends {
		total += b.Weight
	}
	if total <= 0 {
		return backends[0]
	}
	n := rand.Intn(total)
	for _, b := range backends {
		n -= b.Weight
		if n < 0 {
			return b
		}
	}
	return backends[0]
}

// RewritePath 把外部路径前缀替换成后端期望的路径。
// 例如 /review/api/x 转发到后端时保留 /api/x。
func RewritePath(prefix, path string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}
