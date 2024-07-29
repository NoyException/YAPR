package store

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SqlStore 关系型数据库存储
type SqlStore struct {
	db *gorm.DB
}

type StringRecord struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"type:text"`
}

func NewSqlStore(t Type, url string) (*SqlStore, error) {
	var db *gorm.DB
	var err error

	switch t {
	case TypeMysql:
		db, err = gorm.Open(mysql.Open(url), &gorm.Config{})
	case TypePostgres:
		db, err = gorm.Open(postgres.Open(url), &gorm.Config{})
	case TypeSqlite:
		db, err = gorm.Open(sqlite.Open(url), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database type: %s", t)
	}

	if err != nil {
		return nil, err
	}

	return &SqlStore{db: db}, nil
}

func (s *SqlStore) Clear() error {
	err := s.db.Exec("DELETE FROM records").Error
	if err != nil {
		return err
	}

	return nil
}

func (s *SqlStore) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}

	return db.Close()
}

func (s *SqlStore) GetString(key string) (string, error) {
	var str string
	err := s.db.Model(&StringRecord{}).Where("key = ?", key).First(&StringRecord{}).Scan(&str).Error
	if err != nil {
		return "", err
	}
	return str, nil
}

func (s *SqlStore) SetString(key, value string) error {
	err := s.db.Model(&StringRecord{}).Where("key = ?", key).Assign(StringRecord{Key: key, Value: value}).FirstOrCreate(&StringRecord{}).Error
	if err != nil {
		return err
	}
	return nil
}
