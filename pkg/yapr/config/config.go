package config

import (
	"gopkg.in/yaml.v3"
	"noy/router/pkg/yapr/core"
	"os"
	"time"
)

type Config struct {
	Etcd  *ETCDConfig  `yaml:"etcd"`
	Redis *RedisConfig `yaml:"redis"`
	Yapr  *core.Config `yaml:"yapr"`
}

// ETCDConfig etcd 集群配置
type ETCDConfig struct {
	Endpoints   []string      `yaml:"endpoints"`   // etcd 的地址
	DialTimeout time.Duration `yaml:"dialTimeout"` // etcd 的连接超时时间
	Username    string        `yaml:"username"`    // etcd 的用户名
	Password    string        `yaml:"password"`    // etcd 的密码
}

// RedisConfig redis 配置
type RedisConfig struct {
	Url string `yaml:"url"` // redis 的地址
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
