package storage

import (
	"context"
	"time"
)

// Storage 是一个通用的存储接口，支持基本的键值操作
// 后续可以为不同的存储后端（如Redis、Memcached等）实现此接口
type Storage interface {
	// Set 设置键值对
	// 参数：ctx 上下文，key 键，value 值，expiration 过期时间
	// 返回值：错误信息
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// Get 获取键对应的值
	// 参数：ctx 上下文，key 键，value 用于存储结果的指针
	// 返回值：错误信息
	Get(ctx context.Context, key string, value interface{}) error

	// Delete 删除键
	// 参数：ctx 上下文，key 键
	// 返回值：错误信息
	Delete(ctx context.Context, key string) error

	// Exists 检查键是否存在
	// 参数：ctx 上下文，key 键
	// 返回值：是否存在，错误信息
	Exists(ctx context.Context, key string) (bool, error)

	// Expire 设置键的过期时间
	// 参数：ctx 上下文，key 键，expiration 过期时间
	// 返回值：是否成功，错误信息
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)

	// TTL 获取键的剩余过期时间
	// 参数：ctx 上下文，key 键
	// 返回值：剩余过期时间，错误信息
	TTL(ctx context.Context, key string) (time.Duration, error)

	// SetNX 仅当键不存在时设置键值对
	// 参数：ctx 上下文，key 键，value 值，expiration 过期时间
	// 返回值：是否成功，错误信息
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)

	// SetEX 设置键值对并指定过期时间
	// 参数：ctx 上下文，key 键，value 值，expiration 过期时间
	// 返回值：错误信息
	SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// BatchSet 批量设置键值对
	// 参数：ctx 上下文，pairs 键值对映射，expiration 过期时间
	// 返回值：错误信息
	BatchSet(ctx context.Context, pairs map[string]interface{}, expiration time.Duration) error

	// BatchGet 批量获取键对应的值
	// 参数：ctx 上下文，keys 键列表，values 用于存储结果的映射
	// 返回值：错误信息
	BatchGet(ctx context.Context, keys []string, values map[string]interface{}) error

	// BatchDelete 批量删除键
	// 参数：ctx 上下文，keys 键列表
	// 返回值：错误信息
	BatchDelete(ctx context.Context, keys []string) error

	// Close 关闭存储连接
	// 返回值：错误信息
	Close() error
}

// Config 是存储配置的基础结构
type Config struct {
	// 存储类型（如redis、memcached等）
	Type string
	// 连接地址
	Addr string
	// 密码
	Password string
	// 数据库编号（如Redis的db）
	DB int
	// 连接池大小
	PoolSize int
	// 连接超时时间
	DialTimeout time.Duration
	// 读取超时时间
	ReadTimeout time.Duration
	// 写入超时时间
	WriteTimeout time.Duration
}

// StorageOption 是存储选项的函数类型
type StorageOption func(*Config)

// WithType 设置存储类型
func WithType(t string) StorageOption {
	return func(c *Config) {
		c.Type = t
	}
}

// WithAddr 设置连接地址
func WithAddr(addr string) StorageOption {
	return func(c *Config) {
		c.Addr = addr
	}
}

// WithPassword 设置密码
func WithPassword(password string) StorageOption {
	return func(c *Config) {
		c.Password = password
	}
}

// WithDB 设置数据库编号
func WithDB(db int) StorageOption {
	return func(c *Config) {
		c.DB = db
	}
}

// WithPoolSize 设置连接池大小
func WithPoolSize(size int) StorageOption {
	return func(c *Config) {
		c.PoolSize = size
	}
}

// WithDialTimeout 设置连接超时时间
func WithDialTimeout(timeout time.Duration) StorageOption {
	return func(c *Config) {
		c.DialTimeout = timeout
	}
}

// WithReadTimeout 设置读取超时时间
func WithReadTimeout(timeout time.Duration) StorageOption {
	return func(c *Config) {
		c.ReadTimeout = timeout
	}
}

// WithWriteTimeout 设置写入超时时间
func WithWriteTimeout(timeout time.Duration) StorageOption {
	return func(c *Config) {
		c.WriteTimeout = timeout
	}
}

// defaultConfig 返回默认的存储配置
func defaultConfig() Config {
	return Config{
		Type:         "redis",
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}
