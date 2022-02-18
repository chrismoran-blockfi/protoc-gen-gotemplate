// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.4
// source: aaa/aaa.proto

package aaa

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type AaaRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Blah string `protobuf:"bytes,1,opt,name=blah,proto3" json:"blah,omitempty"`
}

func (x *AaaRequest) Reset() {
	*x = AaaRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_aaa_aaa_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AaaRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AaaRequest) ProtoMessage() {}

func (x *AaaRequest) ProtoReflect() protoreflect.Message {
	mi := &file_aaa_aaa_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AaaRequest.ProtoReflect.Descriptor instead.
func (*AaaRequest) Descriptor() ([]byte, []int) {
	return file_aaa_aaa_proto_rawDescGZIP(), []int{0}
}

func (x *AaaRequest) GetBlah() string {
	if x != nil {
		return x.Blah
	}
	return ""
}

type AaaReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Error string `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *AaaReply) Reset() {
	*x = AaaReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_aaa_aaa_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AaaReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AaaReply) ProtoMessage() {}

func (x *AaaReply) ProtoReflect() protoreflect.Message {
	mi := &file_aaa_aaa_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AaaReply.ProtoReflect.Descriptor instead.
func (*AaaReply) Descriptor() ([]byte, []int) {
	return file_aaa_aaa_proto_rawDescGZIP(), []int{1}
}

func (x *AaaReply) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

var File_aaa_aaa_proto protoreflect.FileDescriptor

var file_aaa_aaa_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x61, 0x61, 0x61, 0x2f, 0x61, 0x61, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0f, 0x74, 0x68, 0x65, 0x2e, 0x61, 0x61, 0x61, 0x2e, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65,
	0x22, 0x20, 0x0a, 0x0a, 0x41, 0x61, 0x61, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12,
	0x0a, 0x04, 0x62, 0x6c, 0x61, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x62, 0x6c,
	0x61, 0x68, 0x22, 0x20, 0x0a, 0x08, 0x41, 0x61, 0x61, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x14,
	0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65,
	0x72, 0x72, 0x6f, 0x72, 0x42, 0x5d, 0x5a, 0x5b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x63, 0x68, 0x72, 0x69, 0x73, 0x6d, 0x6f, 0x72, 0x61, 0x6e, 0x2d, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x66, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e,
	0x2d, 0x67, 0x6f, 0x74, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x2f, 0x65, 0x78, 0x61, 0x6d,
	0x70, 0x6c, 0x65, 0x73, 0x2f, 0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2d, 0x70, 0x61, 0x63, 0x6b,
	0x61, 0x67, 0x65, 0x2d, 0x6d, 0x6f, 0x64, 0x65, 0x2f, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x2f,
	0x61, 0x61, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_aaa_aaa_proto_rawDescOnce sync.Once
	file_aaa_aaa_proto_rawDescData = file_aaa_aaa_proto_rawDesc
)

func file_aaa_aaa_proto_rawDescGZIP() []byte {
	file_aaa_aaa_proto_rawDescOnce.Do(func() {
		file_aaa_aaa_proto_rawDescData = protoimpl.X.CompressGZIP(file_aaa_aaa_proto_rawDescData)
	})
	return file_aaa_aaa_proto_rawDescData
}

var file_aaa_aaa_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_aaa_aaa_proto_goTypes = []interface{}{
	(*AaaRequest)(nil), // 0: the.aaa.package.AaaRequest
	(*AaaReply)(nil),   // 1: the.aaa.package.AaaReply
}
var file_aaa_aaa_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_aaa_aaa_proto_init() }
func file_aaa_aaa_proto_init() {
	if File_aaa_aaa_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_aaa_aaa_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AaaRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_aaa_aaa_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AaaReply); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_aaa_aaa_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_aaa_aaa_proto_goTypes,
		DependencyIndexes: file_aaa_aaa_proto_depIdxs,
		MessageInfos:      file_aaa_aaa_proto_msgTypes,
	}.Build()
	File_aaa_aaa_proto = out.File
	file_aaa_aaa_proto_rawDesc = nil
	file_aaa_aaa_proto_goTypes = nil
	file_aaa_aaa_proto_depIdxs = nil
}
