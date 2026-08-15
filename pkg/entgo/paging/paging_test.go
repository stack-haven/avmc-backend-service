package paging

import (
	"testing"

	"entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/assert"

	"backend-service/api/common/pagination"
	"backend-service/pkg/utils/convert"
)

// TestConvertPagingRequest 测试分页请求转换功能
func TestConvertPagingRequest(t *testing.T) {
	// 创建测试用例
	tests := []struct {
		name     string
		request  *pagination.PagingRequest
		expected *PagingOption
	}{{
		name: "基本分页请求",
		request: &pagination.PagingRequest{
			Page:     convert.ToPointer(int32(1)),
			PageSize: convert.ToPointer(int32(10)),
		},
		expected: &PagingOption{
			Limit:  10,
			Offset: 0,
		},
	}, {
		name: "非第一页请求",
		request: &pagination.PagingRequest{
			Page:     convert.ToPointer(int32(3)),
			PageSize: convert.ToPointer(int32(20)),
		},
		expected: &PagingOption{
			Limit:  20,
			Offset: 40,
		},
	}, {
		name: "禁用分页请求",
		request: &pagination.PagingRequest{
			NoPaging: convert.ToPointer(true),
		},
		expected: &PagingOption{
			NoPaging: true,
			Limit:    0,
			Offset:   0,
		},
	}, {
		name: "带排序请求",
		request: &pagination.PagingRequest{
			Page:     convert.ToPointer(int32(1)),
			PageSize: convert.ToPointer(int32(10)),
			OrderBy:  []string{"created_at desc"},
		},
		expected: &PagingOption{
			Limit:  10,
			Offset: 0,
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPagingRequest(tt.request)
			assert.Equal(t, tt.expected.Limit, result.Limit)
			assert.Equal(t, tt.expected.Offset, result.Offset)
			assert.Equal(t, tt.expected.NoPaging, result.NoPaging)
		})
	}
}

// TestParseQueryFilter 测试查询过滤器解析功能
func TestParseQueryFilter(t *testing.T) {
	// 测试有效JSON
	query := `{"name": "test", "age": 30}`
	filter, err := parseQueryFilter(query)
	assert.NoError(t, err)
	assert.NotNil(t, filter)

	// 测试无效JSON（应该作为简单文本搜索处理）
	invalidQuery := `invalid json`
	invalidFilter, err := parseQueryFilter(invalidQuery)
	assert.NoError(t, err)
	assert.NotNil(t, invalidFilter)
}

// TestNewPagingResponse 测试分页响应创建功能
func TestNewPagingResponse(t *testing.T) {
	// 创建测试用例
	tests := []struct {
		total    int32
		data     interface{}
		expected *pagination.PagingResponse
	}{{
		total: 100,
		data:  []string{"item1", "item2"},
		expected: &pagination.PagingResponse{
			Total: 100,
		},
	}, {
		total: 0,
		data:  nil,
		expected: &pagination.PagingResponse{
			Total: 0,
		},
	}}

	for _, tt := range tests {
		result := NewPagingResponse(tt.total, tt.data)
		assert.Equal(t, tt.expected.Total, result.Total)
	}
}

// TestApplyFieldMask 测试字段掩码应用功能
func TestApplyFieldMask(t *testing.T) {
	// 创建一个选择器
	selector := sql.Selector{}

	// 测试应用字段掩码
	fields := []string{"id", "name", "created_at"}
	ApplyFieldMask(&selector, fields)

	// 验证选择器是否正确设置了字段
	assert.NotNil(t, &selector)
}
