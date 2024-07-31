package config

import (
	"gopkg.in/yaml.v3"
	"noy/router/pkg/yapr/core"
	"os"
	"time"
)

type Config struct {
	// etcd 集群配置
	Etcd *ETCDConfig `yaml:"etcd"`
	// redis 集群配置
	Redis *RedisConfig `yaml:"redis"`
	Yapr  *core.Config `yaml:"yapr"`
}

// ETCDConfig etcd 集群配置
type ETCDConfig struct {
	// etcd 的地址
	Endpoints []string `yaml:"endpoints"`
	// etcd 的连接超时时间
	DialTimeout time.Duration `yaml:"dialTimeout"`
	// etcd 的用户名
	Username string `yaml:"username"`
	// etcd 的密码
	Password string `yaml:"password"`
}

type RedisConfig struct {
	// redis 的地址
	Url string `yaml:"url"`
}

// ElectionConfig 选举配置
type ElectionConfig struct {
	// 选举路径
	Path string `yaml:"path"`
	// 选举超时时间
	TTL int `yaml:"ttl"`
}

// LBConfig 负载均衡器配置
type LBConfig struct {
	// 负载均衡器类型
	Type string `yaml:"type"`
	// 重试次数
	Retry int `yaml:"retry"`
}

// BatchLimitConfig 批量上报配置
type BatchLimitConfig struct {
	// 批量上报的最大任务数
	Count int `yaml:"count"`
	// 批量上报的最大时间间隔(秒)
	Interval int `yaml:"interval"`
}

// LoadConfig 加载配置
func LoadConfig(configPath string) (*Config, error) {
	var cfg Config
	var yamlBytes []byte

	if b, err := os.ReadFile(configPath); err != nil {
		return nil, err
	} else {
		// 扩充环境变量
		yamlBytes = []byte(os.ExpandEnv(string(b)))
	}

	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
