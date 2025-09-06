package paging

import (
	"fmt"

	"entgo.io/ent/dialect/sql"

	"backend-service/api/common/pagination"
)

// ExampleUsage 展示如何使用分页封装
func ExampleUsage() {
	// 1. 创建一个分页请求
	pageReq := &pagination.PagingRequest{
		Page:     int32Ptr(1),                             // 第1页
		PageSize: int32Ptr(10),                            // 每页10条
		Query:    strPtr(`{"status": 1, "type": "user"}`), // JSON格式查询条件
		OrderBy:  []string{"created_at desc"},             // 按创建时间降序
		NoPaging: boolPtr(false),                          // 启用分页
		// FieldMask: 可以设置要返回的字段
	}

	// 2. 添加自定义过滤条件
	customFilter := func(s *sql.Selector) {
		s.Where(sql.GT("age", 18))
	}

	// 3. 转换为 PagingOption
	opt := ConvertPagingRequest(pageReq)

	// 4. 添加自定义过滤条件到选项中
	opt.Filters = append(opt.Filters, customFilter)

	// 5. 执行查询 (这里是示例，实际使用时替换为你的 ent 查询)
	// 假设我们有一个 User 实体的查询
	// userQuery := ent.User.Query()
	// userQuery = ApplyOption(userQuery, opt)
	// users, err := userQuery.All(ctx)
	// if err != nil {
	// 	log.Error("Query users failed", "error", err)
	// 	return
	// }

	// 6. 获取总数 (同样是示例)
	// total, err := ent.User.Query().Count(ctx)
	// if err != nil {
	// 	log.Error("Count users failed", "error", err)
	// 	return
	// }

	// 7. 创建分页响应
	total := int32(100)                   // 假设总共有100条记录
	resp := NewPagingResponse(total, nil) // 实际使用时传入查询结果

	// 8. 输出结果示例
	fmt.Printf("分页结果: 总数=%d, 每页条数=%d, 当前页码=%d\n",
		resp.GetTotal(), opt.Limit, (opt.Offset/opt.Limit)+1)

	// 9. 应用字段掩码的示例
	// 假设你有一个 sql.Selector
	// selector := sql.Selector{}
	// ApplyFieldMask(&selector, []string{"id", "name", "created_at"})
}

// ExampleWithCustomConditions 展示如何使用自定义条件
func ExampleWithCustomConditions() {
	// 1. 创建基础分页请求
	pageReq := &pagination.PagingRequest{
		Page:     int32Ptr(1),
		PageSize: int32Ptr(20),
	}

	// 2. 转换为 PagingOption
	opt := ConvertPagingRequest(pageReq)

	// 3. 创建多个自定义过滤条件
	// 条件1: 状态为活跃
	statusFilter := func(s *sql.Selector) {
		s.Where(sql.EQ("status", 1))
	}

	// 条件2: 创建时间在7天内
	// timeFilter := func(s *sql.Selector) {
	// 	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	// 	s.Where(sql.GT("created_at", sevenDaysAgo))
	// }

	// 条件3: 按名称和创建时间排序
	sortByNameAsc := func(s *sql.Selector) {
		s.OrderBy(sql.Asc("name"))
	}

	sortByCreatedAtDesc := func(s *sql.Selector) {
		s.OrderBy(sql.Desc("created_at"))
	}

	// 4. 添加所有条件到选项中
	opt.Filters = append(opt.Filters, statusFilter)
	// opt.Filters = append(opt.Filters, timeFilter)
	opt.Order = append(opt.Order, sortByNameAsc, sortByCreatedAtDesc)

	// 5. 使用选项执行查询
	// 假设我们有一个 Department 实体的查询
	// deptQuery := ent.Department.Query()
	// deptQuery = ApplyOption(deptQuery, opt)
	// departments, err := deptQuery.All(ctx)
	// if err != nil {
	// 	log.Error("Query departments failed", "error", err)
	// 	return
	// }

	// 输出示例信息
	fmt.Println("应用了自定义条件的查询示例")
}

// ExampleWithTextSearch 展示如何使用文本搜索功能
func ExampleWithTextSearch() {
	// 1. 创建包含文本搜索的分页请求
	pageReq := &pagination.PagingRequest{
		Page:     int32Ptr(1),
		PageSize: int32Ptr(10),
		Query:    strPtr("测试部门"), // 简单文本搜索
	}

	// 2. 转换为 PagingOption
	opt := ConvertPagingRequest(pageReq)

	// 3. 执行查询
	// 假设我们有一个 Department 实体的查询
	// deptQuery := ent.Department.Query()
	// deptQuery = ApplyOption(deptQuery, opt)
	// departments, err := deptQuery.All(ctx)
	// if err != nil {
	// 	log.Error("Query departments failed", "error", err)
	// 	return
	// }

	// 输出示例信息
	fmt.Println("使用文本搜索的查询示例", opt)
}

// ExampleWithOrConditions 展示如何使用OR条件查询
func ExampleWithOrConditions() {
	// 1. 创建包含OR条件的分页请求
	pageReq := &pagination.PagingRequest{
		Page:     int32Ptr(1),
		PageSize: int32Ptr(10),
		OrQuery:  strPtr(`{"type": "admin", "type": "super_admin"}`), // OR条件
	}

	// 2. 转换为 PagingOption
	opt := ConvertPagingRequest(pageReq)

	// 3. 执行查询
	// 假设我们有一个 User 实体的查询
	// userQuery := ent.User.Query()
	// userQuery = ApplyOption(userQuery, opt)
	// users, err := userQuery.All(ctx)
	// if err != nil {
	// 	log.Error("Query users failed", "error", err)
	// 	return
	// }

	// 输出示例信息
	fmt.Println("使用OR条件的查询示例", opt)
}
