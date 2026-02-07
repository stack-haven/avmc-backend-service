package memcached

import (
	"backend-service/pkg/storage"
	"context"
	"encoding/json"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

// MemcachedStorage 是Memcached存储的实现
type MemcachedStorage struct {
	client *memcache.Client
}

// NewMemcachedStorage 创建一个新的Memcached存储实例
// 参数：config 存储配置
// 返回值：存储实例，错误信息
func NewMemcachedStorage(config storage.Config) (*MemcachedStorage, error) {
	client := memcache.New(config.Addr)

	// 测试连接
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 由于memcache客户端没有Ping方法，我们使用Set和Get来测试连接
	testKey := "test_connection"
	testValue := []byte("test")
	if err := client.Set(&memcache.Item{
		Key:        testKey,
		Value:      testValue,
		Expiration: 1,
	}); err != nil {
		return nil, err
	}

	// 清理测试键
	client.Delete(testKey)

	return &MemcachedStorage{
		client: client,
	}, nil
}

// Set 设置键值对
func (s *MemcachedStorage) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return s.client.Set(&memcache.Item{
		Key:        key,
		Value:      data,
		Expiration: int32(expiration.Seconds()),
	})
}

// Get 获取键对应的值
func (s *MemcachedStorage) Get(ctx context.Context, key string, value interface{}) error {
	// 获取值
	item, err := s.client.Get(key)
	if err != nil {
		return err
	}

	// 反序列化值
	return json.Unmarshal(item.Value, value)
}

// Delete 删除键
func (s *MemcachedStorage) Delete(ctx context.Context, key string) error {
	return s.client.Delete(key)
}

// Exists 检查键是否存在
func (s *MemcachedStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.Get(key)
	if err == memcache.ErrCacheMiss {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Expire 设置键的过期时间
func (s *MemcachedStorage) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	// 先获取值
	item, err := s.client.Get(key)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return false, nil
		}
		return false, err
	}

	// 重新设置值，指定新的过期时间
	err = s.client.Set(&memcache.Item{
		Key:        key,
		Value:      item.Value,
		Expiration: int32(expiration.Seconds()),
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

// TTL 获取键的剩余过期时间
func (s *MemcachedStorage) TTL(ctx context.Context, key string) (time.Duration, error) {
	// Memcached不支持直接获取键的剩余过期时间
	// 这里我们返回一个默认值，表示不支持此操作
	return 0, nil
}

// SetNX 仅当键不存在时设置键值对
func (s *MemcachedStorage) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	// 检查键是否存在
	exists, err := s.Exists(ctx, key)
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	// 设置键值对
	err = s.client.Set(&memcache.Item{
		Key:        key,
		Value:      data,
		Expiration: int32(expiration.Seconds()),
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetEX 设置键值对并指定过期时间
func (s *MemcachedStorage) SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return s.client.Set(&memcache.Item{
		Key:        key,
		Value:      data,
		Expiration: int32(expiration.Seconds()),
	})
}

// BatchSet 批量设置键值对
func (s *MemcachedStorage) BatchSet(ctx context.Context, pairs map[string]interface{}, expiration time.Duration) error {
	for key, value := range pairs {
		// 序列化值
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}

		s.client.Set(&memcache.Item{
			Key:        key,
			Value:      data,
			Expiration: int32(expiration.Seconds()),
		})
	}

	return nil
}

// BatchGet 批量获取键对应的值
func (s *MemcachedStorage) BatchGet(ctx context.Context, keys []string, values map[string]interface{}) error {
	items, err := s.client.GetMulti(keys)
	if err != nil {
		return err
	}

	for key, item := range items {
		// 反序列化值
		var value interface{}
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return err
		}

		values[key] = value
	}

	return nil
}

// BatchDelete 批量删除键
func (s *MemcachedStorage) BatchDelete(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := s.client.Delete(key); err != nil && err != memcache.ErrCacheMiss {
			return err
		}
	}

	return nil
}

// Close 关闭存储连接
func (s *MemcachedStorage) Close() error {
	// Memcached客户端没有Close方法，这里我们返回nil
	return nil
}
