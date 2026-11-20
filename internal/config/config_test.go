package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 回归用例：默认值必须落到切片元素上。
// 以前 validate 按值遍历 routes/backends，默认值写进副本就丢了：
// 没配 breaker 的路由阈值 0、窗口 0 秒，熔断每拍都秒过半开，等于永不拦截；
// 没配 weight 的后端权重全是 0，PickBackend 里 total<=0 直接退化到第一个后端。
func TestValidateDefaultsReachSliceElements(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gw.yaml")
	yaml := `listen: ":9000"
auth:
  enabled: false
routes:
  - prefix: "/a"
    backends:
      - url: "http://127.0.0.1:1"
      - url: "http://127.0.0.1:2"
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("写临时配置失败: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := cfg.Routes[0]
	if r.Breaker.FailureThreshold != 5 {
		t.Errorf("breaker.failure_threshold 默认值未生效: 得到 %d，期望 5", r.Breaker.FailureThreshold)
	}
	if r.Breaker.OpenSeconds != 10 {
		t.Errorf("breaker.open_seconds 默认值未生效: 得到 %d，期望 10", r.Breaker.OpenSeconds)
	}
	for i, b := range r.Backends {
		if b.Weight != 1 {
			t.Errorf("第 %d 个后端 weight 默认值未生效: 得到 %d，期望 1", i+1, b.Weight)
		}
	}
}

// 显式配置必须原样保留，不能被默认值覆盖。
func TestValidateKeepsExplicitValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gw.yaml")
	yaml := `listen: ":9000"
auth:
  enabled: false
routes:
  - prefix: "/b"
    backends:
      - url: "http://127.0.0.1:1"
        weight: 7
    breaker:
      failure_threshold: 2
      open_seconds: 30
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("写临时配置失败: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := cfg.Routes[0]
	if r.Breaker.FailureThreshold != 2 || r.Breaker.OpenSeconds != 30 {
		t.Errorf("显式 breaker 被覆盖: 得到 %d/%d，期望 2/30", r.Breaker.FailureThreshold, r.Breaker.OpenSeconds)
	}
	if r.Backends[0].Weight != 7 {
		t.Errorf("显式 weight 被覆盖: 得到 %d，期望 7", r.Backends[0].Weight)
	}
}
