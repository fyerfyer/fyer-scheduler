package utils

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Query 封装MongoDB查询条件构建器
type Query struct {
	filter     bson.M
	projection bson.M
	sort       bson.D
	skip       int64
	limit      int64
}

// NewQuery 创建一个新的查询构建器
func NewQuery() *Query {
	return &Query{
		filter:     bson.M{},
		projection: bson.M{},
		sort:       bson.D{},
		skip:       0,
		limit:      0,
	}
}

// Where 添加等值条件
func (q *Query) Where(field string, value interface{}) *Query {
	q.filter[field] = value
	return q
}

// WhereIn 添加包含条件
func (q *Query) WhereIn(field string, values []interface{}) *Query {
	q.filter[field] = bson.M{"$in": values}
	return q
}

// WhereGt 添加大于条件
func (q *Query) WhereGt(field string, value interface{}) *Query {
	q.filter[field] = bson.M{"$gt": value}
	return q
}

// WhereLt 添加小于条件
func (q *Query) WhereLt(field string, value interface{}) *Query {
	q.filter[field] = bson.M{"$lt": value}
	return q
}

// WhereGte 添加大于等于条件
func (q *Query) WhereGte(field string, value interface{}) *Query {
	q.filter[field] = bson.M{"$gte": value}
	return q
}

// WhereLte 添加小于等于条件
func (q *Query) WhereLte(field string, value interface{}) *Query {
	q.filter[field] = bson.M{"$lte": value}
	return q
}

// WhereNe 添加不等于条件
func (q *Query) WhereNe(field string, value interface{}) *Query {
	q.filter[field] = bson.M{"$ne": value}
	return q
}

// WhereRegex 添加正则表达式条件
func (q *Query) WhereRegex(field string, pattern string) *Query {
	q.filter[field] = bson.M{"$regex": pattern}
	return q
}

// WhereBetween 添加范围条件
func (q *Query) WhereBetween(field string, min, max interface{}) *Query {
	q.filter[field] = bson.M{"$gte": min, "$lte": max}
	return q
}

// WhereExists 添加字段存在条件
func (q *Query) WhereExists(field string, exists bool) *Query {
	q.filter[field] = bson.M{"$exists": exists}
	return q
}

// Select 指定要返回的字段
func (q *Query) Select(fields ...string) *Query {
	for _, field := range fields {
		q.projection[field] = 1
	}
	return q
}

// Exclude 指定要排除的字段
func (q *Query) Exclude(fields ...string) *Query {
	for _, field := range fields {
		q.projection[field] = 0
	}
	return q
}

// OrderBy 添加排序条件
func (q *Query) OrderBy(field string, ascending bool) *Query {
	var order int
	if ascending {
		order = 1
	} else {
		order = -1
	}
	q.sort = append(q.sort, bson.E{Key: field, Value: order})
	return q
}

// Skip 设置跳过的文档数
func (q *Query) Skip(n int64) *Query {
	q.skip = n
	return q
}

// Limit 设置返回的最大文档数
func (q *Query) Limit(n int64) *Query {
	q.limit = n
	return q
}

// SetPage 设置分页
func (q *Query) SetPage(page, pageSize int64) *Query {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	q.skip = (page - 1) * pageSize
	q.limit = pageSize
	return q
}

// GetFilter 获取过滤条件
func (q *Query) GetFilter() bson.M {
	return q.filter
}

// GetOptions 获取查询选项
func (q *Query) GetOptions() *options.FindOptions {
	opts := options.Find()

	// 设置投影
	if len(q.projection) > 0 {
		opts.SetProjection(q.projection)
	}

	// 设置排序
	if len(q.sort) > 0 {
		opts.SetSort(q.sort)
	}

	// 设置分页
	if q.skip > 0 {
		opts.SetSkip(q.skip)
	}
	if q.limit > 0 {
		opts.SetLimit(q.limit)
	}

	return opts
}

// Reset 重置查询条件
func (q *Query) Reset() *Query {
	q.filter = bson.M{}
	q.projection = bson.M{}
	q.sort = bson.D{}
	q.skip = 0
	q.limit = 0
	return q
}
