package store

import (
	"fmt"
	"log"
)

// Store 是每个存储后端都需要实现的接口
type Store interface {
	GetString(key string) (string, error)
	SetString(key, value string) error
	// Close 关闭存储
	Close() error
}

var (
	// _ Store = (*MemoryStore)(nil)
	_ Store = (*SqlStore)(nil)
	// _ Store = (*RedisStore)(nil)
	_ Store = (*MongoStore)(nil)
)

func Init(cfg *Config) (Store, error) {
	if cfg == nil {
		log.Println("[store] config is nil")
		cfg = &Config{}
	}
	var store Store
	var err error

	switch cfg.Type {
	case TypeSqlite, TypeMysql, TypePostgres:
		store, err = NewSqlStore(cfg.Type, cfg.URL)
		if err != nil {
			return store, err
		}
	case TypeMongo:
		store, err = NewMongoStore(cfg.URL)
		if err != nil {
			return store, err
		}
	default:
		return nil, fmt.Errorf("unsupported store type: %s", cfg.Type)
	}

	return store, err
}
