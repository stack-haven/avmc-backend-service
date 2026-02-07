package redis

import (
	"backend-service/pkg/storage"
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage 是Redis存储的实现
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage 创建一个新的Redis存储实例
// 参数：config 存储配置
// 返回值：存储实例，错误信息
func NewRedisStorage(config storage.Config) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStorage{
		client: client,
	}, nil
}

// Set 设置键值对
func (s *RedisStorage) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, key, data, expiration).Err()
}

// Get 获取键对应的值
func (s *RedisStorage) Get(ctx context.Context, key string, value interface{}) error {
	// 获取值
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}

	// 反序列化值
	return json.Unmarshal(data, value)
}

// Delete 删除键
func (s *RedisStorage) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// Exists 检查键是否存在
func (s *RedisStorage) Exists(ctx context.Context, key string) (bool, error) {
	result, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return result > 0, nil
}

// Expire 设置键的过期时间
func (s *RedisStorage) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return s.client.Expire(ctx, key, expiration).Result()
}

// TTL 获取键的剩余过期时间
func (s *RedisStorage) TTL(ctx context.Context, key string) (time.Duration, error) {
	return s.client.TTL(ctx, key).Result()
}

// SetNX 仅当键不存在时设置键值对
func (s *RedisStorage) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	return s.client.SetNX(ctx, key, data, expiration).Result()
}

// SetEX 设置键值对并指定过期时间
func (s *RedisStorage) SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return s.client.SetEx(ctx, key, data, expiration).Err()
}

// BatchSet 批量设置键值对
func (s *RedisStorage) BatchSet(ctx context.Context, pairs map[string]interface{}, expiration time.Duration) error {
	// 使用管道批量操作
	pipe := s.client.Pipeline()

	for key, value := range pairs {
		// 序列化值
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}

		pipe.Set(ctx, key, data, expiration)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// BatchGet 批量获取键对应的值
func (s *RedisStorage) BatchGet(ctx context.Context, keys []string, values map[string]interface{}) error {
	// 使用管道批量操作
	pipe := s.client.Pipeline()

	// 存储每个键的结果通道
	cmds := make(map[string]*redis.StringCmd)
	for _, key := range keys {
		cmds[key] = pipe.Get(ctx, key)
	}

	// 执行管道操作
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return err
	}

	// 处理结果
	for key, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil && err != redis.Nil {
			return err
		}

		if err == redis.Nil {
			// 键不存在，跳过
			continue
		}

		// 反序列化值
		var value interface{}
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}

		values[key] = value
	}

	return nil
}

// BatchDelete 批量删除键
func (s *RedisStorage) BatchDelete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	return s.client.Del(ctx, keys...).Err()
}

// Close 关闭存储连接
func (s *RedisStorage) Close() error {
	return s.client.Close()
}

// 注册Redis存储到NewStorage函数
func init() {
	// 后续可以通过修改NewStorage函数来支持Redis存储
}
