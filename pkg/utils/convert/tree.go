package convert

import (
	"errors"
)

// Pid 定义支持的父 ID 类型
type Pid interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

// TreeData 定义树节点数据接口
type TreeData[T Pid] interface {
	GetId() T
	GetParentId() T
}

// ToTree 将扁平数据转换为树形结构
// source: 原始数据切片
// pid: 根节点的父 ID
// addChildren: 用于为节点添加子节点的函数
func ToTree[T TreeData[I], I Pid](source []T, pid I, addChildren func(T, ...T) error) ([]T, error) {
	result := make([]T, 0)
	// 缓存所有节点，方便快速查找
	nodeMap := make(map[I]T)
	for _, item := range source {
		id := item.GetId()
		nodeMap[id] = item
	}

	for _, item := range source {
		parentId := item.GetParentId()
		if parentId == pid {
			result = append(result, item)
		} else if parent, exists := nodeMap[parentId]; exists {
			if err := addChildren(parent, item); err != nil {
				return nil, errors.New("failed to add child to parent: " + err.Error())
			}
		}
	}

	// 递归处理子节点
	for _, item := range result {
		children, err := ToTree(source, item.GetId(), addChildren)
		if err != nil {
			return nil, err
		}
		if len(children) > 0 {
			if err := addChildren(item, children...); err != nil {
				return nil, errors.New("failed to add children to node: " + err.Error())
			}
		}
	}

	return result, nil
}
