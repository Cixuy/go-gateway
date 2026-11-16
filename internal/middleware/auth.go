package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Auth 校验 Authorization: Bearer <token>，白名单路径直接放行。
type Auth struct {
	secret    []byte
	enabled   bool
	whitelist []string

	// 由配置热加载更新
	Update func(enabled bool, secret string, whitelist []string)
}

func NewAuth(enabled bool, secret string, whitelist []string) *Auth {
	a := &Auth{enabled: enabled, secret: []byte(secret), whitelist: whitelist}
	a.Update = func(e bool, s string, w []string) {
		a.enabled, a.secret, a.whitelist = e, []byte(s), w
	}
	return a
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.enabled || a.allowed(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
			// 只接受 HMAC 系列，防止 alg=none 这类降级攻击
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return a.secret, nil
		})
		if err != nil || !parsed.Valid {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
			if sub, ok := claims["sub"].(string); ok {
				r.Header.Set("X-User", sub)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Auth) allowed(path string) bool {
	for _, p := range a.whitelist {
		if p == path || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
			return true
		}
	}
	return false
}
