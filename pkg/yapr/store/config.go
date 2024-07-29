package store

// 存储后端类型
type Type string

const (
	// 存储后端类型为 SQLite
	TypeSqlite Type = "sqlite"

	// 存储后端类型为 MySQL
	TypeMysql Type = "mysql"

	// 存储后端类型为 PostgreSQL
	TypePostgres Type = "postgres"

	// // 存储后端类型为内存
	// TypeMemory Type = "memory"

	// // 存储后端类型为 Redis
	// TypeRedis Type = "redis"

	// 存储后端类型为 MongoDB
	TypeMongo Type = "mongo"
)

// Config 存储后端的相关配置
type Config struct {
	// 存储后端类型
	Type Type `yaml:"type"`

	// 存储后端的地址，如 mongodb://localhost:27017
	URL string `yaml:"url"`
}
