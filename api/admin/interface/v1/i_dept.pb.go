// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        (unknown)
// source: admin/interface/v1/i_dept.proto

package v1

import (
	v1 "backend-service/api/admin/service/v1"
	pagination "backend-service/api/common/pagination"
	_ "github.com/google/gnostic/openapiv3"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	_ "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

var File_admin_interface_v1_i_dept_proto protoreflect.FileDescriptor

var file_admin_interface_v1_i_dept_proto_rawDesc = string([]byte{
	0x0a, 0x1f, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x66, 0x61, 0x63,
	0x65, 0x2f, 0x76, 0x31, 0x2f, 0x69, 0x5f, 0x64, 0x65, 0x70, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x12, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x66, 0x61,
	0x63, 0x65, 0x2e, 0x76, 0x31, 0x1a, 0x24, 0x67, 0x6e, 0x6f, 0x73, 0x74, 0x69, 0x63, 0x2f, 0x6f,
	0x70, 0x65, 0x6e, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x33, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x22, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2f, 0x70,
	0x61, 0x67, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x70, 0x61, 0x67, 0x69, 0x6e, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x61, 0x64, 0x6d, 0x69,
	0x6e, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x76, 0x31, 0x2f, 0x64, 0x65, 0x70,
	0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0xe0, 0x05, 0x0a, 0x0b, 0x44, 0x65, 0x70, 0x74,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x81, 0x01, 0x0a, 0x08, 0x4c, 0x69, 0x73, 0x74,
	0x44, 0x65, 0x70, 0x74, 0x12, 0x19, 0x2e, 0x70, 0x61, 0x67, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x50, 0x61, 0x67, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x22, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e,
	0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x65, 0x70, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x36, 0xba, 0x47, 0x1c, 0x5a, 0x1a, 0x0a, 0x18, 0x0a, 0x14, 0x4f, 0x41,
	0x75, 0x74, 0x68, 0x32, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x42, 0x65, 0x61, 0x72,
	0x65, 0x72, 0x12, 0x00, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x11, 0x12, 0x0f, 0x2f, 0x61, 0x64, 0x6d,
	0x69, 0x6e, 0x2f, 0x76, 0x31, 0x2f, 0x64, 0x65, 0x70, 0x74, 0x73, 0x12, 0x80, 0x01, 0x0a, 0x07,
	0x47, 0x65, 0x74, 0x44, 0x65, 0x70, 0x74, 0x12, 0x20, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x44, 0x65,
	0x70, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x61, 0x64, 0x6d, 0x69,
	0x6e, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x44, 0x65, 0x70,
	0x74, 0x22, 0x3b, 0xba, 0x47, 0x1c, 0x5a, 0x1a, 0x0a, 0x18, 0x0a, 0x14, 0x4f, 0x41, 0x75, 0x74,
	0x68, 0x32, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x42, 0x65, 0x61, 0x72, 0x65, 0x72,
	0x12, 0x00, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x16, 0x12, 0x14, 0x2f, 0x61, 0x64, 0x6d, 0x69, 0x6e,
	0x2f, 0x76, 0x31, 0x2f, 0x64, 0x65, 0x70, 0x74, 0x73, 0x2f, 0x7b, 0x69, 0x64, 0x7d, 0x12, 0x95,
	0x01, 0x0a, 0x0a, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x44, 0x65, 0x70, 0x74, 0x12, 0x23, 0x2e,
	0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x76, 0x31,
	0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x44, 0x65, 0x70, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x24, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x44, 0x65, 0x70, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x3c, 0xba, 0x47, 0x1c, 0x5a, 0x1a, 0x0a,
	0x18, 0x0a, 0x14, 0x4f, 0x41, 0x75, 0x74, 0x68, 0x32, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72,
	0x64, 0x42, 0x65, 0x61, 0x72, 0x65, 0x72, 0x12, 0x00, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x17, 0x3a,
	0x04, 0x64, 0x65, 0x70, 0x74, 0x22, 0x0f, 0x2f, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2f, 0x76, 0x31,
	0x2f, 0x64, 0x65, 0x70, 0x74, 0x73, 0x12, 0x9a, 0x01, 0x0a, 0x0a, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x44, 0x65, 0x70, 0x74, 0x12, 0x23, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x44,
	0x65, 0x70, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x24, 0x2e, 0x61, 0x64, 0x6d,
	0x69, 0x6e, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x44, 0x65, 0x70, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x41, 0xba, 0x47, 0x1c, 0x5a, 0x1a, 0x0a, 0x18, 0x0a, 0x14, 0x4f, 0x41, 0x75, 0x74, 0x68,
	0x32, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x42, 0x65, 0x61, 0x72, 0x65, 0x72, 0x12,
	0x00, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x1c, 0x3a, 0x04, 0x64, 0x65, 0x70, 0x74, 0x1a, 0x14, 0x2f,
	0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2f, 0x76, 0x31, 0x2f, 0x64, 0x65, 0x70, 0x74, 0x73, 0x2f, 0x7b,
	0x69, 0x64, 0x7d, 0x12, 0x94, 0x01, 0x0a, 0x0a, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x44, 0x65,
	0x70, 0x74, 0x12, 0x23, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x44, 0x65, 0x70, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x24, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74,
	0x65, 0x44, 0x65, 0x70, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x3b, 0xba,
	0x47, 0x1c, 0x5a, 0x1a, 0x0a, 0x18, 0x0a, 0x14, 0x4f, 0x41, 0x75, 0x74, 0x68, 0x32, 0x50, 0x61,
	0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x42, 0x65, 0x61, 0x72, 0x65, 0x72, 0x12, 0x00, 0x82, 0xd3,
	0xe4, 0x93, 0x02, 0x16, 0x2a, 0x14, 0x2f, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2f, 0x76, 0x31, 0x2f,
	0x64, 0x65, 0x70, 0x74, 0x73, 0x2f, 0x7b, 0x69, 0x64, 0x7d, 0x42, 0xbb, 0x01, 0x0a, 0x16, 0x63,
	0x6f, 0x6d, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x66, 0x61,
	0x63, 0x65, 0x2e, 0x76, 0x31, 0x42, 0x0a, 0x49, 0x44, 0x65, 0x70, 0x74, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x50, 0x01, 0x5a, 0x29, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2d, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2f, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x66, 0x61, 0x63, 0x65, 0x2f, 0x76, 0x31, 0x3b, 0x76, 0x31, 0xa2, 0x02,
	0x03, 0x41, 0x49, 0x58, 0xaa, 0x02, 0x12, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x49, 0x6e, 0x74,
	0x65, 0x72, 0x66, 0x61, 0x63, 0x65, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x13, 0x41, 0x64, 0x6d, 0x69,
	0x6e, 0x5c, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x66, 0x61, 0x63, 0x65, 0x5f, 0x5c, 0x56, 0x31, 0xe2,
	0x02, 0x1f, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x5c, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x66, 0x61, 0x63,
	0x65, 0x5f, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74,
	0x61, 0xea, 0x02, 0x14, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x3a, 0x3a, 0x49, 0x6e, 0x74, 0x65, 0x72,
	0x66, 0x61, 0x63, 0x65, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var file_admin_interface_v1_i_dept_proto_goTypes = []any{
	(*pagination.PagingRequest)(nil), // 0: pagination.PagingRequest
	(*v1.GetDeptRequest)(nil),        // 1: admin.service.v1.GetDeptRequest
	(*v1.CreateDeptRequest)(nil),     // 2: admin.service.v1.CreateDeptRequest
	(*v1.UpdateDeptRequest)(nil),     // 3: admin.service.v1.UpdateDeptRequest
	(*v1.DeleteDeptRequest)(nil),     // 4: admin.service.v1.DeleteDeptRequest
	(*v1.ListDeptResponse)(nil),      // 5: admin.service.v1.ListDeptResponse
	(*v1.Dept)(nil),                  // 6: admin.service.v1.Dept
	(*v1.CreateDeptResponse)(nil),    // 7: admin.service.v1.CreateDeptResponse
	(*v1.UpdateDeptResponse)(nil),    // 8: admin.service.v1.UpdateDeptResponse
	(*v1.DeleteDeptResponse)(nil),    // 9: admin.service.v1.DeleteDeptResponse
}
var file_admin_interface_v1_i_dept_proto_depIdxs = []int32{
	0, // 0: admin.interface.v1.DeptService.ListDept:input_type -> pagination.PagingRequest
	1, // 1: admin.interface.v1.DeptService.GetDept:input_type -> admin.service.v1.GetDeptRequest
	2, // 2: admin.interface.v1.DeptService.CreateDept:input_type -> admin.service.v1.CreateDeptRequest
	3, // 3: admin.interface.v1.DeptService.UpdateDept:input_type -> admin.service.v1.UpdateDeptRequest
	4, // 4: admin.interface.v1.DeptService.DeleteDept:input_type -> admin.service.v1.DeleteDeptRequest
	5, // 5: admin.interface.v1.DeptService.ListDept:output_type -> admin.service.v1.ListDeptResponse
	6, // 6: admin.interface.v1.DeptService.GetDept:output_type -> admin.service.v1.Dept
	7, // 7: admin.interface.v1.DeptService.CreateDept:output_type -> admin.service.v1.CreateDeptResponse
	8, // 8: admin.interface.v1.DeptService.UpdateDept:output_type -> admin.service.v1.UpdateDeptResponse
	9, // 9: admin.interface.v1.DeptService.DeleteDept:output_type -> admin.service.v1.DeleteDeptResponse
	5, // [5:10] is the sub-list for method output_type
	0, // [0:5] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_admin_interface_v1_i_dept_proto_init() }
func file_admin_interface_v1_i_dept_proto_init() {
	if File_admin_interface_v1_i_dept_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_admin_interface_v1_i_dept_proto_rawDesc), len(file_admin_interface_v1_i_dept_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_admin_interface_v1_i_dept_proto_goTypes,
		DependencyIndexes: file_admin_interface_v1_i_dept_proto_depIdxs,
	}.Build()
	File_admin_interface_v1_i_dept_proto = out.File
	file_admin_interface_v1_i_dept_proto_goTypes = nil
	file_admin_interface_v1_i_dept_proto_depIdxs = nil
}
