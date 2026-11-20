package config

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是网关的全部可调参数。
type Config struct {
	Listen    string        `yaml:"listen"`
	Auth      AuthConfig    `yaml:"auth"`
	RateLimit RateConfig    `yaml:"rate_limit"`
	Routes    []RouteConfig `yaml:"routes"`
	Log       LogConfig     `yaml:"log"`
}

type AuthConfig struct {
	Enabled   bool     `yaml:"enabled"`
	JWTSecret string   `yaml:"jwt_secret"`
	Whitelist []string `yaml:"whitelist"`
}

type RateConfig struct {
	RPS   float64 `yaml:"rps"`
	Burst int     `yaml:"burst"`
}

type RouteConfig struct {
	Prefix       string          `yaml:"prefix"`
	Backends     []BackendConfig `yaml:"backends"`
	AuthRequired *bool           `yaml:"auth_required"`
	Breaker      BreakerConfig   `yaml:"breaker"`
}

type BackendConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

type BreakerConfig struct {
	FailureThreshold int `yaml:"failure_threshold"`
	OpenSeconds      int `yaml:"open_seconds"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

// Load 读取并校验配置文件。校验失败直接返回错误，避免带着错误配置启动。
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Listen == "" {
		return errors.New("listen 不能为空")
	}
	if c.Auth.Enabled && c.Auth.JWTSecret == "" {
		return errors.New("开启鉴权时必须配置 jwt_secret")
	}
	if c.RateLimit.RPS <= 0 {
		c.RateLimit.RPS = 50
	}
	if c.RateLimit.Burst <= 0 {
		c.RateLimit.Burst = 100
	}
	if len(c.Routes) == 0 {
		return errors.New("至少要配置一条路由")
	}
	// 注意必须用下标取址：按值遍历拿到的是副本，默认值写回副本等于没写，
	// 于是没填 breaker 的路由阈值/窗口都是 0，熔断永不拦截；没填 weight 的后端权重恒为 0，
	// PickBackend 里 total<=0 直接退化到第一个后端，多实例分流形同虚设。
	for i := range c.Routes {
		r := &c.Routes[i]
		if r.Prefix == "" {
			return fmt.Errorf("第 %d 条路由缺少 prefix", i+1)
		}
		if len(r.Backends) == 0 {
			return fmt.Errorf("路由 %s 没有配置后端", r.Prefix)
		}
		for j := range r.Backends {
			b := &r.Backends[j]
			if b.URL == "" {
				return fmt.Errorf("路由 %s 的后端缺少 url", r.Prefix)
			}
			if b.Weight <= 0 {
				b.Weight = 1
			}
		}
		if r.Breaker.FailureThreshold <= 0 {
			r.Breaker.FailureThreshold = 5
		}
		if r.Breaker.OpenSeconds <= 0 {
			r.Breaker.OpenSeconds = 10
		}
	}
	return nil
}

// Manager 负责保存当前配置并在文件变化时热加载。
type Manager struct {
	path    string
	mu      sync.RWMutex
	cfg     *Config
	modTime time.Time
}

func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	info, _ := os.Stat(path)
	m := &Manager{path: path, cfg: cfg}
	if info != nil {
		m.modTime = info.ModTime()
	}
	return m, nil
}

func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Watch 每两秒检查一次文件修改时间，变了就重新加载；加载失败保留旧配置。
func (m *Manager) Watch(interval time.Duration, onReload func(*Config)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			info, err := os.Stat(m.path)
			if err != nil {
				continue
			}
			m.mu.RLock()
			changed := info.ModTime().After(m.modTime)
			m.mu.RUnlock()
			if !changed {
				continue
			}
			cfg, err := Load(m.path)
			if err != nil {
				fmt.Printf("[config] 重新加载失败，继续使用旧配置: %v\n", err)
				continue
			}
			m.mu.Lock()
			m.cfg = cfg
			m.modTime = info.ModTime()
			m.mu.Unlock()
			fmt.Println("[config] 配置已重新加载")
			if onReload != nil {
				onReload(cfg)
			}
		}
	}()
}
