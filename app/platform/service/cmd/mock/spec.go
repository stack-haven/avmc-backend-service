//go:build mock

package main

// mockMenuSpec 定义菜单目录/页面的 seed 规格。
// Type: 1=目录(FOLDER) 2=菜单(MENU)，按钮(3)由 buttonSpec 单独定义。
type mockMenuSpec struct {
	Name, Title, Path, Component, Icon string
	Type, Sort                         int32
	AuthCode                           string
}

// menuSeed 定义菜单目录/页面的 seed 规格（平铺 + Parent 引用）。
// 列表按“父在前、子在后”顺序书写，seed 时按顺序建立 name→ID 映射。
type menuSeed struct {
	Parent    string // 父菜单 name（空表示一级目录）
	Name      string // 菜单 name（唯一键）
	Title     string // 菜单中文标题
	Path      string // 前端路由路径
	Component string // 前端组件
	Icon      string // 图标
	Type      int32  // 1=目录 2=页面
	Sort      int32
}

// buttonSpec 定义权限按钮（菜单 type=3）的 seed 规格。
// Operation 字段直接引用 api 生成的 v1.OperationXxx 常量，
// 编译期即可校验接口名称，避免手写字符串拼写错误；后续新增接口时
// 只需在对应模块的 Buttons 列表中追加一行，即可增量维护。
type buttonSpec struct {
	Parent    string // 父菜单 name（唯一键）
	Name      string // 按钮 name（唯一键，用于幂等 upsert）
	Title     string // 按钮中文标题
	Operation string // v1.OperationXxx（HTTP 接口 operation 常量）
	Sort      int32  // 按钮排序
}
