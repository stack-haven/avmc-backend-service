package convert

import (
	"fmt"
)

// Pid 定义支持的父 ID 类型
type Pid interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

// TreeData 定义树节点数据接口
type TreeItem[T Pid] interface {
	GetId() T
	GetParentId() T
}

// ToTree：一次遍历挂子节点，O(n)
func ToTree[T TreeItem[I], I Pid](
	source []T,
	rootPid I,
	addChildren func(T, ...T) error,
) ([]T, error) {
	nodeMap := make(map[I]T, len(source))
	for i := range source {
		// 存指针，避免拷贝
		node := source[i]
		id := node.GetId() // 注意：对 *T 先解引用再调用
		nodeMap[id] = node
	}

	var roots []T
	for i := range source {
		node := source[i]
		pid := node.GetParentId()
		if pid == rootPid {
			roots = append(roots, node)
			continue
		}
		if parent, ok := nodeMap[pid]; ok {
			if err := addChildren(parent, node); err != nil {
				return nil, fmt.Errorf("failed to add child: %w", err)
			}
		}
	}
	return roots, nil
}

// ToTreeWith 将扁平化的切片数据转换为树形结构。
// - T: 节点类型（任意 struct）
// - I: 节点 ID 类型（约束为 Pid）
//
// 参数说明：
//
//	source     : 原始节点切片（扁平数据）
//	rootPid    : 根节点的父 ID（通常为 0 或 -1）
//	idOf       : 获取节点 ID 的函数
//	parentOf   : 获取节点父 ID 的函数
//	addChildren: 将子节点追加到父节点的函数
//
// 返回：
//
//	[]*T       : 树的根节点切片（可能有多棵树）
//	error      : 错误信息
func ToTreeWith[T any, I Pid](source []*T, rootPid I, idOf func(*T) I, parentOf func(*T) I, addChildren func(*T, ...*T) error,
) ([]*T, error) {
	// 缓存 id -> 节点指针，方便 O(1) 查找
	nodeMap := make(map[I]*T, len(source))
	for i := range source {
		node := &source[i]
		nodeMap[idOf(*node)] = *node
	}

	// 存放最终的根节点
	roots := make([]*T, 0, len(source))

	// 挂接子节点
	for i := range source {
		node := &source[i]
		pid := parentOf(*node)
		if pid == rootPid {
			// 根节点
			roots = append(roots, *node)
			continue
		}
		// 找到父节点，挂到父节点下
		if parent, ok := nodeMap[pid]; ok {
			if err := addChildren(parent, *node); err != nil {
				return nil, fmt.Errorf("failed to add child: %w", err)
			}
		}
	}

	return roots, nil
}
