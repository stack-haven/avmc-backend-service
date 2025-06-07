package convert

import (
	"testing"
)

// 租户模块
type Domain struct {
	Id          uint64    `protobuf:"varint,3,opt,name=id,proto3" json:"id,omitempty"`
	Name        string    `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	ParentId    uint64    `protobuf:"varint,5,opt,name=parent_id,json=parentId,proto3" json:"parent_id,omitempty"`
	Code        *string   `protobuf:"bytes,6,opt,name=code,proto3,oneof" json:"code,omitempty"`
	Sort        *int32    `protobuf:"varint,7,opt,name=sort,proto3,oneof" json:"sort,omitempty"`
	Alias       *string   `protobuf:"bytes,8,opt,name=alias,proto3,oneof" json:"alias,omitempty"`
	Logo        *string   `protobuf:"bytes,9,opt,name=logo,proto3,oneof" json:"logo,omitempty"`
	Pic         *string   `protobuf:"bytes,10,opt,name=pic,proto3,oneof" json:"pic,omitempty"`
	Keywords    *string   `protobuf:"bytes,11,opt,name=keywords,proto3,oneof" json:"keywords,omitempty"`
	Description *string   `protobuf:"bytes,12,opt,name=description,proto3,oneof" json:"description,omitempty"`
	State       *int32    `protobuf:"varint,13,opt,name=state,proto3,oneof" json:"state,omitempty"`
	Remarks     *string   `protobuf:"bytes,14,opt,name=remarks,proto3,oneof" json:"remarks,omitempty"`
	Children    []*Domain `protobuf:"bytes,15,rep,name=children,proto3" json:"children,omitempty"`
}

func (x *Domain) GetId() uint64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Domain) GetParentId() uint64 {
	if x != nil {
		return x.ParentId
	}
	return 0
}

type DomainTest struct {
	*Domain
}

var domainData = []*Domain{
	{
		Id:       1,
		Name:     "测试1",
		ParentId: 0,
	}, {
		Id:       2,
		Name:     "测试2",
		ParentId: 0,
	}, {
		Id:       3,
		Name:     "测试3",
		ParentId: 1,
	}, {
		Id:       4,
		Name:     "测试4",
		ParentId: 1,
	}, {
		Id:       5,
		Name:     "测试5",
		ParentId: 2,
	}, {
		Id:       6,
		Name:     "测试6",
		ParentId: 0,
	},
}

func (d *DomainTest) AppendChildren(arg any) {
	domains, ok := arg.([]*DomainTest)
	if !ok {
		return
	}
	l := make([]*Domain, 0, len(domains))
	for _, v := range domains {
		l = append(l, v.Domain)
	}
	d.Children = append(d.Children, l...)
}

func Test_TreeProto(t *testing.T) {
	t.Log("测试泛型树形结构开始")
	data, err := ToTree(domainData, 0, func(t *Domain, ts ...*Domain) error {
		t.Children = append(t.Children, ts...)
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v", data)
	t.Log("测试泛型树形结构结束")
}
