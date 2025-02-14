// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        (unknown)
// source: common/conf/base.proto

package conf

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Base struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Admin         *Base_Admin            `protobuf:"bytes,1,opt,name=admin,proto3" json:"admin,omitempty"`
	Domain        *Base_Domain           `protobuf:"bytes,2,opt,name=domain,proto3" json:"domain,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Base) Reset() {
	*x = Base{}
	mi := &file_common_conf_base_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Base) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Base) ProtoMessage() {}

func (x *Base) ProtoReflect() protoreflect.Message {
	mi := &file_common_conf_base_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Base.ProtoReflect.Descriptor instead.
func (*Base) Descriptor() ([]byte, []int) {
	return file_common_conf_base_proto_rawDescGZIP(), []int{0}
}

func (x *Base) GetAdmin() *Base_Admin {
	if x != nil {
		return x.Admin
	}
	return nil
}

func (x *Base) GetDomain() *Base_Domain {
	if x != nil {
		return x.Domain
	}
	return nil
}

// Admin 管理配置
type Base_Admin struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	DomainId      uint64                 `protobuf:"varint,1,opt,name=domain_id,json=domainId,proto3" json:"domain_id,omitempty"`
	UserId        uint64                 `protobuf:"varint,2,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Base_Admin) Reset() {
	*x = Base_Admin{}
	mi := &file_common_conf_base_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Base_Admin) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Base_Admin) ProtoMessage() {}

func (x *Base_Admin) ProtoReflect() protoreflect.Message {
	mi := &file_common_conf_base_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Base_Admin.ProtoReflect.Descriptor instead.
func (*Base_Admin) Descriptor() ([]byte, []int) {
	return file_common_conf_base_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Base_Admin) GetDomainId() uint64 {
	if x != nil {
		return x.DomainId
	}
	return 0
}

func (x *Base_Admin) GetUserId() uint64 {
	if x != nil {
		return x.UserId
	}
	return 0
}

// Domain
type Base_Domain struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Base_Domain) Reset() {
	*x = Base_Domain{}
	mi := &file_common_conf_base_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Base_Domain) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Base_Domain) ProtoMessage() {}

func (x *Base_Domain) ProtoReflect() protoreflect.Message {
	mi := &file_common_conf_base_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Base_Domain.ProtoReflect.Descriptor instead.
func (*Base_Domain) Descriptor() ([]byte, []int) {
	return file_common_conf_base_proto_rawDescGZIP(), []int{0, 1}
}

var File_common_conf_base_proto protoreflect.FileDescriptor

var file_common_conf_base_proto_rawDesc = string([]byte{
	0x0a, 0x16, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x2f, 0x62, 0x61,
	0x73, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x63, 0x6f, 0x6e, 0x66, 0x22, 0xa2,
	0x01, 0x0a, 0x04, 0x42, 0x61, 0x73, 0x65, 0x12, 0x26, 0x0a, 0x05, 0x61, 0x64, 0x6d, 0x69, 0x6e,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x2e, 0x42, 0x61,
	0x73, 0x65, 0x2e, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x52, 0x05, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x12,
	0x29, 0x0a, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x11, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x2e, 0x42, 0x61, 0x73, 0x65, 0x2e, 0x44, 0x6f, 0x6d, 0x61,
	0x69, 0x6e, 0x52, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x1a, 0x3d, 0x0a, 0x05, 0x41, 0x64,
	0x6d, 0x69, 0x6e, 0x12, 0x1b, 0x0a, 0x09, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x5f, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x49, 0x64,
	0x12, 0x17, 0x0a, 0x07, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x06, 0x75, 0x73, 0x65, 0x72, 0x49, 0x64, 0x1a, 0x08, 0x0a, 0x06, 0x44, 0x6f, 0x6d,
	0x61, 0x69, 0x6e, 0x42, 0x6b, 0x0a, 0x08, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x42,
	0x09, 0x42, 0x61, 0x73, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x24, 0x62, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2d, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x61, 0x70,
	0x69, 0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x3b, 0x63, 0x6f,
	0x6e, 0x66, 0xa2, 0x02, 0x03, 0x43, 0x58, 0x58, 0xaa, 0x02, 0x04, 0x43, 0x6f, 0x6e, 0x66, 0xca,
	0x02, 0x04, 0x43, 0x6f, 0x6e, 0x66, 0xe2, 0x02, 0x10, 0x43, 0x6f, 0x6e, 0x66, 0x5c, 0x47, 0x50,
	0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x04, 0x43, 0x6f, 0x6e, 0x66,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_common_conf_base_proto_rawDescOnce sync.Once
	file_common_conf_base_proto_rawDescData []byte
)

func file_common_conf_base_proto_rawDescGZIP() []byte {
	file_common_conf_base_proto_rawDescOnce.Do(func() {
		file_common_conf_base_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_common_conf_base_proto_rawDesc), len(file_common_conf_base_proto_rawDesc)))
	})
	return file_common_conf_base_proto_rawDescData
}

var file_common_conf_base_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_common_conf_base_proto_goTypes = []any{
	(*Base)(nil),        // 0: conf.Base
	(*Base_Admin)(nil),  // 1: conf.Base.Admin
	(*Base_Domain)(nil), // 2: conf.Base.Domain
}
var file_common_conf_base_proto_depIdxs = []int32{
	1, // 0: conf.Base.admin:type_name -> conf.Base.Admin
	2, // 1: conf.Base.domain:type_name -> conf.Base.Domain
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_common_conf_base_proto_init() }
func file_common_conf_base_proto_init() {
	if File_common_conf_base_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_common_conf_base_proto_rawDesc), len(file_common_conf_base_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_common_conf_base_proto_goTypes,
		DependencyIndexes: file_common_conf_base_proto_depIdxs,
		MessageInfos:      file_common_conf_base_proto_msgTypes,
	}.Build()
	File_common_conf_base_proto = out.File
	file_common_conf_base_proto_goTypes = nil
	file_common_conf_base_proto_depIdxs = nil
}
