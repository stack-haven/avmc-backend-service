package paging

import (
	"fmt"
)

// ExampleFix 展示如何正确使用修改后的ApplyOption函数
// 当ApplyOption返回interface{}类型时，需要使用类型断言将其转换回具体的查询类型
func ExampleFix() {
	// 假设我们有以下代码（用户原始代码）：
	// query := paging.ApplyOption(r.data.DB(ctx).Role.Query(), opt)
	// roles, err := query.All(ctx)
	// 
	// 由于ApplyOption现在返回interface{}类型，我们需要使用类型断言
	// 来将其转换回原始的查询类型
	
	// 修复方法：使用类型断言
	// 注意：这里的*gen.RoleQuery应该替换为实际的查询类型
	// ctx := context.Background()
	// opt := paging.ConvertPagingRequest(pagingReq)
	// rawQuery := paging.ApplyOption(r.data.DB(ctx).Role.Query(), opt)
	// 
	// // 类型断言
	// query, ok := rawQuery.(*gen.RoleQuery)
	// if !ok {
	// 	// 处理类型断言失败的情况
	// 	return nil, fmt.Errorf("无法将查询结果转换为RoleQuery类型")
	// }
	// 
	// // 现在可以正常使用查询方法
	// roles, err := query.All(ctx)

	// 另一种更简洁的写法
	// roles, err := rawQuery.(interface{ All(context.Context) ([]*gen.Role, error) }).All(ctx)

	fmt.Println("请根据上述示例修复您的代码中的类型断言问题")
}