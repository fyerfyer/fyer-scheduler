package utils

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// MongoDBClient 是对MongoDB客户端的封装
type MongoDBClient struct {
	client   *mongo.Client
	database *mongo.Database
	config   *config.MongoDBConfig
}

// NewMongoDBClient 创建一个新的MongoDB客户端
func NewMongoDBClient(cfg *config.MongoDBConfig) (*MongoDBClient, error) {
	// 创建客户端
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.URI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// 测试连接
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	// 获取数据库引用
	database := client.Database(cfg.Database)

	return &MongoDBClient{
		client:   client,
		database: database,
		config:   cfg,
	}, nil
}

// Close 关闭MongoDB连接
func (m *MongoDBClient) Close() error {
	if m.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return m.client.Disconnect(ctx)
	}
	return nil
}

// withRetry 执行带重试的操作
func (m *MongoDBClient) withRetry(operation func() error, description string) error {
	var err error
	maxRetries := 3
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}

		// 记录重试信息
		Warn("MongoDB operation failed, retrying",
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

// GetCollection 获取指定的集合
func (m *MongoDBClient) GetCollection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

// InsertOne 插入单个文档
func (m *MongoDBClient) InsertOne(collection string, document interface{}) (interface{}, error) {
	var result interface{}

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		res, err := coll.InsertOne(ctx, document)
		if err != nil {
			return fmt.Errorf("failed to insert document: %w", err)
		}

		result = res.InsertedID
		return nil
	}

	err := m.withRetry(operation, "InsertOne: "+collection)
	return result, err
}

// InsertMany 插入多个文档
func (m *MongoDBClient) InsertMany(collection string, documents []interface{}) ([]interface{}, error) {
	var result []interface{}

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		res, err := coll.InsertMany(ctx, documents)
		if err != nil {
			return fmt.Errorf("failed to insert documents: %w", err)
		}

		result = res.InsertedIDs
		return nil
	}

	err := m.withRetry(operation, "InsertMany: "+collection)
	return result, err
}

// FindOne 查找单个文档
func (m *MongoDBClient) FindOne(collection string, filter interface{}, result interface{}) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		err := coll.FindOne(ctx, filter).Decode(result)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return fmt.Errorf("document not found")
			}
			return fmt.Errorf("failed to find document: %w", err)
		}

		return nil
	}

	return m.withRetry(operation, "FindOne: "+collection)
}

// FindMany 查找多个文档
func (m *MongoDBClient) FindMany(collection string, filter interface{}, results interface{}) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		cursor, err := coll.Find(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to find documents: %w", err)
		}
		defer cursor.Close(ctx)

		if err := cursor.All(ctx, results); err != nil {
			return fmt.Errorf("failed to decode documents: %w", err)
		}

		return nil
	}

	return m.withRetry(operation, "FindMany: "+collection)
}

// FindWithOptions 带选项的查询（支持分页、排序等）
func (m *MongoDBClient) FindWithOptions(
	collection string,
	filter interface{},
	results interface{},
	opts ...*options.FindOptions,
) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		cursor, err := coll.Find(ctx, filter, opts...)
		if err != nil {
			return fmt.Errorf("failed to find documents: %w", err)
		}
		defer cursor.Close(ctx)

		if err := cursor.All(ctx, results); err != nil {
			return fmt.Errorf("failed to decode documents: %w", err)
		}

		return nil
	}

	return m.withRetry(operation, "FindWithOptions: "+collection)
}

// UpdateOne 更新单个文档
func (m *MongoDBClient) UpdateOne(collection string, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	var result *mongo.UpdateResult

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		res, err := coll.UpdateOne(ctx, filter, update, opts...)
		if err != nil {
			return fmt.Errorf("failed to update document: %w", err)
		}

		result = res
		return nil
	}

	err := m.withRetry(operation, "UpdateOne: "+collection)
	return result, err
}

// UpdateMany 更新多个文档
func (m *MongoDBClient) UpdateMany(collection string, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	var result *mongo.UpdateResult

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		res, err := coll.UpdateMany(ctx, filter, update, opts...)
		if err != nil {
			return fmt.Errorf("failed to update documents: %w", err)
		}

		result = res
		return nil
	}

	err := m.withRetry(operation, "UpdateMany: "+collection)
	return result, err
}

// DeleteOne 删除单个文档
func (m *MongoDBClient) DeleteOne(collection string, filter interface{}) (*mongo.DeleteResult, error) {
	var result *mongo.DeleteResult

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		res, err := coll.DeleteOne(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to delete document: %w", err)
		}

		result = res
		return nil
	}

	err := m.withRetry(operation, "DeleteOne: "+collection)
	return result, err
}

// DeleteMany 删除多个文档
func (m *MongoDBClient) DeleteMany(collection string, filter interface{}) (*mongo.DeleteResult, error) {
	var result *mongo.DeleteResult

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		res, err := coll.DeleteMany(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to delete documents: %w", err)
		}

		result = res
		return nil
	}

	err := m.withRetry(operation, "DeleteMany: "+collection)
	return result, err
}

// Count 获取文档数量
func (m *MongoDBClient) Count(collection string, filter interface{}) (int64, error) {
	var count int64

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		c, err := coll.CountDocuments(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to count documents: %w", err)
		}

		count = c
		return nil
	}

	err := m.withRetry(operation, "Count: "+collection)
	return count, err
}

// CreateIndex 创建索引
func (m *MongoDBClient) CreateIndex(collection string, keys bson.D, unique bool) (string, error) {
	var indexName string

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		indexModel := mongo.IndexModel{
			Keys:    keys,
			Options: options.Index().SetUnique(unique),
		}

		name, err := coll.Indexes().CreateOne(ctx, indexModel)
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}

		indexName = name
		return nil
	}

	err := m.withRetry(operation, "CreateIndex: "+collection)
	return indexName, err
}

// Aggregate 执行聚合查询
func (m *MongoDBClient) Aggregate(collection string, pipeline interface{}, results interface{}) error {
	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		coll := m.GetCollection(collection)
		cursor, err := coll.Aggregate(ctx, pipeline)
		if err != nil {
			return fmt.Errorf("failed to execute aggregate query: %w", err)
		}
		defer cursor.Close(ctx)

		if err := cursor.All(ctx, results); err != nil {
			return fmt.Errorf("failed to decode aggregate results: %w", err)
		}

		return nil
	}

	return m.withRetry(operation, "Aggregate: "+collection)
}
