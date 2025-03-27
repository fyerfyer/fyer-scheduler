package utils

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/config"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdClient 是对etcd客户端的封装
type EtcdClient struct {
	client *clientv3.Client
	config *config.EtcdConfig
}

// NewEtcdClient 创建一个新的etcd客户端
func NewEtcdClient(cfg *config.EtcdConfig) (*EtcdClient, error) {
	// 创建etcd客户端配置
	clientConfig := clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: time.Duration(constants.EtcdDialTimeout) * time.Second,
	}

	// 如果提供了认证信息
	if cfg.Username != "" && cfg.Password != "" {
		clientConfig.Username = cfg.Username
		clientConfig.Password = cfg.Password
	}

	// 创建客户端
	client, err := clientv3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdDialTimeout)*time.Second)
	defer cancel()

	_, err = client.Status(ctx, cfg.Endpoints[0])
	if err != nil {
		return nil, fmt.Errorf("failed to check etcd status: %w", err)
	}

	return &EtcdClient{
		client: client,
		config: cfg,
	}, nil
}

// Close 关闭etcd连接
func (e *EtcdClient) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

// withRetry 执行带重试的操作
func (e *EtcdClient) withRetry(operation func() error, description string) error {
	var err error
	maxRetries := 3
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}

		// 记录重试信息
		Warn("etcd operation failed, retrying",
			zap.String("operation", description),
			zap.Int("attempt", attempt+1),
			zap.Error(err))

		// 计算下一次重试延迟（指数退避 + 随机抖动）
		jitter := time.Duration(rand.Int63n(int64(retryDelay) / 2))
		sleepTime := retryDelay + jitter
		time.Sleep(sleepTime)

		// 增加下一次重试的延迟时间
		retryDelay *= 2
	}

	// 所有重试都失败
	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, err)
}

// Get 获取键的值
func (e *EtcdClient) Get(key string) (string, error) {
	var result string

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		resp, err := e.client.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get key from etcd: %w", err)
		}

		if len(resp.Kvs) == 0 {
			result = ""
			return nil // 键不存在
		}

		result = string(resp.Kvs[0].Value)
		return nil
	}

	err := e.withRetry(operation, "Get: "+key)
	return result, err
}

// GetWithPrefix 获取带前缀的所有键值对
func (e *EtcdClient) GetWithPrefix(prefix string) (map[string]string, error) {
	var result map[string]string

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())
		if err != nil {
			return fmt.Errorf("failed to get keys with prefix from etcd: %w", err)
		}

		result = make(map[string]string)
		for _, kv := range resp.Kvs {
			result[string(kv.Key)] = string(kv.Value)
		}

		return nil
	}

	Info("fetching keys with prefix", zap.String("prefix", prefix))
	err := e.withRetry(operation, "GetWithPrefix: "+prefix)
	if err != nil {
		Error("failed to fetch keys with prefix", zap.String("prefix", prefix), zap.Error(err))
		return nil, err
	}
	return result, err
}

// Put 保存键值对
func (e *EtcdClient) Put(key, value string) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		_, err := e.client.Put(ctx, key, value)
		if err != nil {
			return fmt.Errorf("failed to put key to etcd: %w", err)
		}
		return nil
	}

	return e.withRetry(operation, "Put: "+key)
}

// PutWithLease 保存带租约的键值对
func (e *EtcdClient) PutWithLease(key, value string, ttl int64) (clientv3.LeaseID, error) {
	var leaseID clientv3.LeaseID

	operation := func() error {
		// 创建租约
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		lease, err := e.client.Grant(ctx, ttl)
		if err != nil {
			return fmt.Errorf("failed to create lease: %w", err)
		}

		// 使用租约保存键值对
		_, err = e.client.Put(ctx, key, value, clientv3.WithLease(lease.ID))
		if err != nil {
			return fmt.Errorf("failed to put key with lease: %w", err)
		}

		leaseID = lease.ID
		return nil
	}

	err := e.withRetry(operation, "PutWithLease: "+key)
	return leaseID, err
}

// Delete 删除键
func (e *EtcdClient) Delete(key string) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		_, err := e.client.Delete(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to delete key from etcd: %w", err)
		}
		return nil
	}

	return e.withRetry(operation, "Delete: "+key)
}

// DeleteWithPrefix 删除带前缀的所有键
func (e *EtcdClient) DeleteWithPrefix(prefix string) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		_, err := e.client.Delete(ctx, prefix, clientv3.WithPrefix())
		if err != nil {
			return fmt.Errorf("failed to delete keys with prefix from etcd: %w", err)
		}
		return nil
	}

	return e.withRetry(operation, "DeleteWithPrefix: "+prefix)
}

// Watch 监听键的变化
func (e *EtcdClient) Watch(key string, handler func(string, string, string)) {
	go func() {
		watchChan := e.client.Watch(context.Background(), key)
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				var eventType string
				switch event.Type {
				case clientv3.EventTypePut:
					eventType = "PUT"
				case clientv3.EventTypeDelete:
					eventType = "DELETE"
				default:
					eventType = "UNKNOWN"
				}

				value := ""
				if event.Type == clientv3.EventTypePut {
					value = string(event.Kv.Value)
				}

				handler(eventType, string(event.Kv.Key), value)
			}
		}

		// 如果watchChan关闭，尝试重新连接
		Error("watch channel closed, trying to reconnect", zap.String("key", key))
		// 短暂延迟后重新连接
		time.Sleep(time.Second)
		e.Watch(key, handler)
	}()
}

// WatchWithPrefix 监听带前缀键的变化
func (e *EtcdClient) WatchWithPrefix(prefix string, handler func(string, string, string)) {
	go func() {
		watchChan := e.client.Watch(context.Background(), prefix, clientv3.WithPrefix())
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				var eventType string
				switch event.Type {
				case clientv3.EventTypePut:
					eventType = "PUT"
				case clientv3.EventTypeDelete:
					eventType = "DELETE"
				default:
					eventType = "UNKNOWN"
				}

				value := ""
				if event.Type == clientv3.EventTypePut {
					value = string(event.Kv.Value)
				}

				handler(eventType, string(event.Kv.Key), value)
			}
		}

		// 如果watchChan关闭，尝试重新连接
		Error("watch channel closed, trying to reconnect", zap.String("prefix", prefix))
		// 短暂延迟后重新连接
		time.Sleep(time.Second)
		e.WatchWithPrefix(prefix, handler)
	}()
}

// TryAcquireLock 尝试获取分布式锁
func (e *EtcdClient) TryAcquireLock(lockKey string, ttl int64) (clientv3.LeaseID, error) {
	var leaseID clientv3.LeaseID

	operation := func() error {
		// 创建一个租约
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		lease, err := e.client.Grant(ctx, ttl)
		if err != nil {
			return fmt.Errorf("failed to create lease: %w", err)
		}

		// 尝试获取锁
		txn := e.client.Txn(ctx)
		txn = txn.If(clientv3.Compare(clientv3.CreateRevision(lockKey), "=", 0))
		txn = txn.Then(clientv3.OpPut(lockKey, "", clientv3.WithLease(lease.ID)))
		txn = txn.Else(clientv3.OpGet(lockKey))

		resp, err := txn.Commit()
		if err != nil {
			return fmt.Errorf("transaction failed: %w", err)
		}

		if !resp.Succeeded {
			// 锁已被占用
			return fmt.Errorf("lock already acquired by another client")
		}

		leaseID = lease.ID
		return nil
	}

	err := e.withRetry(operation, "TryAcquireLock: "+lockKey)
	return leaseID, err
}

// ReleaseLock 释放分布式锁
func (e *EtcdClient) ReleaseLock(leaseID clientv3.LeaseID) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.EtcdOpTimeout)*time.Second)
		defer cancel()

		_, err := e.client.Revoke(ctx, leaseID)
		if err != nil {
			return fmt.Errorf("failed to revoke lease: %w", err)
		}
		return nil
	}

	return e.withRetry(operation, fmt.Sprintf("ReleaseLock: %d", leaseID))
}

// KeepAliveLock 保持锁的活跃状态
func (e *EtcdClient) KeepAliveLock(leaseID clientv3.LeaseID) error {
	ch, err := e.client.KeepAlive(context.Background(), leaseID)
	if err != nil {
		return fmt.Errorf("failed to keep lease alive: %w", err)
	}

	go func() {
		for {
			ka, ok := <-ch
			if !ok {
				Info("lease keep alive channel closed", zap.Int64("leaseID", int64(leaseID)))
				return
			}
			Debug("keeping lease alive", zap.Int64("leaseID", int64(leaseID)), zap.Int64("ttl", ka.TTL))
		}
	}()

	return nil
}

// KeepAliveLease 启动租约自动续期，返回响应channel
func (e *EtcdClient) KeepAliveLease(ctx context.Context, leaseID clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	return e.client.KeepAlive(ctx, leaseID)
}
