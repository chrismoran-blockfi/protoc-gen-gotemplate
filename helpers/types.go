package helpers

import (
	"fmt"
	"github.com/huandu/xstrings"
	"strconv"
	"strings"

	descriptor "google.golang.org/protobuf/types/descriptorpb"
)

// A GoImportPath is the import path of a Go package. e.g., "google.golang.org/genproto/protobuf".
type GoImportPath string

func (p GoImportPath) String() string { return strconv.Quote(string(p)) }

// A GoPackageName is the name of a Go package. e.g., "protobuf".
type GoPackageName string

// GoPackage represents a golang package
type GoPackage struct {
	// Path is the package path to the package.
	Path string
	// Name is the package name of the package
	Name string
	// Alias is an alias of the package unique within the current invokation of grpc-gateway generator.
	Alias string
}

// Standard returns whether the import is a golang standard package.
func (p GoPackage) Standard() bool {
	return !strings.Contains(p.Path, ".")
}

// String returns a string representation of this package in the form of import line in golang.
func (p GoPackage) String() string {
	if p.Alias == "" {
		return fmt.Sprintf("%q", p.Path)
	}
	return fmt.Sprintf("%s %q", p.Alias, p.Path)
}

// File wraps descriptor.FileDescriptorProto for richer features.
type File struct {
	*descriptor.FileDescriptorProto
	// GoPkg is the go package of the go file generated from this file..
	GoPkg GoPackage
	// Messages is the list of messages defined in this file.
	Messages []*Message
	// Enums is the list of enums defined in this file.
	Enums []*Enum
	// Services is the list of services defined in this file.
	Services []*Service
	// Directives is the mappings of elements to comment-directives in this file
	Directives map[interface{}][]CommentDirective
}

type CommentDirective struct {
	Directive string
	Params    string
	Value     string
	Type      string
}

// proto2 determines if the syntax of the file is proto2.
func (f *File) proto2() bool {
	return f.Syntax == nil || f.GetSyntax() == "proto2"
}

// Message describes a protocol buffer message types
type Message struct {
	// File is the file where the message is defined
	File *File
	// Outers is a list of outer messages if this message is a nested type.
	Outers []string
	*descriptor.DescriptorProto
	Fields []*Field

	// Index is proto path index of this message in File.
	Index int
}

// FQMN returns a fully qualified message name of this message.
func (m *Message) FQMN() string {
	components := []string{""}
	if m.File.Package != nil {
		components = append(components, m.File.GetPackage())
	}
	components = append(components, m.Outers...)
	components = append(components, m.GetName())
	return strings.Join(components, ".")
}

// GoType returns a go type name for the message type.
// It prefixes the type name with the package alias if
// its belonging package is not "currentPackage".
func (m *Message) GoType(currentPackage string) string {
	var components []string
	components = append(components, m.Outers...)
	components = append(components, m.GetName())

	name := strings.Join(components, "_")
	if m.File.GoPkg.Path == currentPackage {
		return name
	}
	pkg := m.File.GoPkg.Name
	if alias := m.File.GoPkg.Alias; alias != "" {
		pkg = alias
	}
	return fmt.Sprintf("%s.%s", pkg, name)
}

// Enum describes a protocol buffer enum types
type Enum struct {
	// File is the file where the enum is defined
	File *File
	// Outers is a list of outer messages if this enum is a nested type.
	Outers []string
	*descriptor.EnumDescriptorProto

	Index int
}

// FQEN returns a fully qualified enum name of this enum.
func (e *Enum) FQEN() string {
	components := []string{""}
	if e.File.Package != nil {
		components = append(components, e.File.GetPackage())
	}
	components = append(components, e.Outers...)
	components = append(components, e.GetName())
	return strings.Join(components, ".")
}

// GoType returns a go type name for the enum type.
// It prefixes the type name with the package alias if
// its belonging package is not "currentPackage".
func (e *Enum) GoType(currentPackage string) string {
	var components []string
	components = append(components, e.Outers...)
	components = append(components, e.GetName())

	name := strings.Join(components, "_")
	if e.File.GoPkg.Path == currentPackage {
		return name
	}
	pkg := e.File.GoPkg.Name
	if alias := e.File.GoPkg.Alias; alias != "" {
		pkg = alias
	}
	return fmt.Sprintf("%s.%s", pkg, name)
}

// Service wraps descriptor.ServiceDescriptorProto for richer features.
type Service struct {
	// File is the file where this service is defined.
	File *File
	*descriptor.ServiceDescriptorProto
	// Methods is the list of methods defined in this service.
	Methods []*Method
}

// FQSN returns the fully qualified service name of this service.
func (s *Service) FQSN() string {
	components := []string{""}
	if s.File.Package != nil {
		components = append(components, s.File.GetPackage())
	}
	components = append(components, s.GetName())
	return strings.Join(components, ".")
}

// Method wraps descriptor.MethodDescriptorProto for richer features.
type Method struct {
	// Service is the service which this method belongs to.
	Service *Service
	*descriptor.MethodDescriptorProto

	// RequestType is the message type of requests to this method.
	RequestType *Message
	// ResponseType is the message type of responses from this method.
	ResponseType *Message
}

// FQMN returns a fully qualified rpc method name of this method.
func (m *Method) FQMN() string {
	components := make([]string, 2)
	components = append(components, m.Service.FQSN())
	components = append(components, m.GetName())
	return strings.Join(components, ".")
}

// Field wraps descriptor.FieldDescriptorProto for richer features.
type Field struct {
	// Message is the message type which this field belongs to.
	Message *Message
	// FieldMessage is the message type of the field.
	FieldMessage *Message
	*descriptor.FieldDescriptorProto
}

// FieldPath is a path to a field from a request message.
type FieldPath []FieldPathComponent

// String returns a string representation of the field path.
func (p FieldPath) String() string {
	var components []string
	for _, c := range p {
		components = append(components, c.Name)
	}
	return strings.Join(components, ".")
}

// IsNestedProto3 indicates whether the FieldPath is a nested Proto3 path.
func (p FieldPath) IsNestedProto3() bool {
	if len(p) > 1 && !p[0].Target.Message.File.proto2() {
		return true
	}
	return false
}

// AssignableExpr is an assignable expression in Go to be used to assign a value to the target field.
// It starts with "msgExpr", which is the go expression of the method request object.
func (p FieldPath) AssignableExpr(msgExpr string) string {
	l := len(p)
	if l == 0 {
		return msgExpr
	}

	var preparations []string
	components := msgExpr
	for i, c := range p {
		// Check if it is a oneOf field.
		if c.Target.OneofIndex != nil {
			index := c.Target.OneofIndex
			msg := c.Target.Message
			oneOfName := xstrings.ToCamelCase(msg.GetOneofDecl()[*index].GetName())
			oneofFieldName := msg.GetName() + "_" + c.AssignableExpr()

			components = components + "." + oneOfName
			s := `if %s == nil {
				%s =&%s{}
			} else if _, ok := %s.(*%s); !ok {
				return nil, metadata, grpc.Errorf(codes.InvalidArgument, "expect type: *%s, but: %%t\n",%s)
			}`

			preparations = append(preparations, fmt.Sprintf(s, components, components, oneofFieldName, components, oneofFieldName, oneofFieldName, components))
			components = components + ".(*" + oneofFieldName + ")"
		}

		if i == l-1 {
			components = components + "." + c.AssignableExpr()
			continue
		}
		components = components + "." + c.ValueExpr()
	}

	preparations = append(preparations, components)
	return strings.Join(preparations, "\n")
}

// FieldPathComponent is a path component in FieldPath
type FieldPathComponent struct {
	// Name is a name of the proto field which this component corresponds to.
	// TODO(yugui) is this necessary?
	Name string
	// Target is the proto field which this component corresponds to.
	Target *Field
}

// AssignableExpr returns an assignable expression in go for this field.
func (c FieldPathComponent) AssignableExpr() string {
	return xstrings.ToCamelCase(c.Name)
}

// ValueExpr returns an expression in go for this field.
func (c FieldPathComponent) ValueExpr() string {
	if c.Target.Message.File.proto2() {
		return fmt.Sprintf("Get%s()", xstrings.ToCamelCase(c.Name))
	}
	return xstrings.ToCamelCase(c.Name)
}

var (
/*
	proto3ConvertFuncs = map[descriptor.FieldDescriptorProto_Type]string{
		descriptor.FieldDescriptorProto_TYPE_DOUBLE:  "runtime.Float64",
		descriptor.FieldDescriptorProto_TYPE_FLOAT:   "runtime.Float32",
		descriptor.FieldDescriptorProto_TYPE_INT64:   "runtime.Int64",
		descriptor.FieldDescriptorProto_TYPE_UINT64:  "runtime.Uint64",
		descriptor.FieldDescriptorProto_TYPE_INT32:   "runtime.Int32",
		descriptor.FieldDescriptorProto_TYPE_FIXED64: "runtime.Uint64",
		descriptor.FieldDescriptorProto_TYPE_FIXED32: "runtime.Uint32",
		descriptor.FieldDescriptorProto_TYPE_BOOL:    "runtime.Bool",
		descriptor.FieldDescriptorProto_TYPE_STRING:  "runtime.String",
		// FieldDescriptorProto_TYPE_GROUP
		// FieldDescriptorProto_TYPE_MESSAGE
		descriptor.FieldDescriptorProto_TYPE_BYTES:    "runtime.Bytes",
		descriptor.FieldDescriptorProto_TYPE_UINT32:   "runtime.Uint32",
		descriptor.FieldDescriptorProto_TYPE_ENUM:     "runtime.Enum",
		descriptor.FieldDescriptorProto_TYPE_SFIXED32: "runtime.Int32",
		descriptor.FieldDescriptorProto_TYPE_SFIXED64: "runtime.Int64",
		descriptor.FieldDescriptorProto_TYPE_SINT32:   "runtime.Int32",
		descriptor.FieldDescriptorProto_TYPE_SINT64:   "runtime.Int64",
	}

	proto3RepeatedConvertFuncs = map[descriptor.FieldDescriptorProto_Type]string{
		descriptor.FieldDescriptorProto_TYPE_DOUBLE:  "runtime.Float64Slice",
		descriptor.FieldDescriptorProto_TYPE_FLOAT:   "runtime.Float32Slice",
		descriptor.FieldDescriptorProto_TYPE_INT64:   "runtime.Int64Slice",
		descriptor.FieldDescriptorProto_TYPE_UINT64:  "runtime.Uint64Slice",
		descriptor.FieldDescriptorProto_TYPE_INT32:   "runtime.Int32Slice",
		descriptor.FieldDescriptorProto_TYPE_FIXED64: "runtime.Uint64Slice",
		descriptor.FieldDescriptorProto_TYPE_FIXED32: "runtime.Uint32Slice",
		descriptor.FieldDescriptorProto_TYPE_BOOL:    "runtime.BoolSlice",
		descriptor.FieldDescriptorProto_TYPE_STRING:  "runtime.StringSlice",
		// FieldDescriptorProto_TYPE_GROUP
		// FieldDescriptorProto_TYPE_MESSAGE
		descriptor.FieldDescriptorProto_TYPE_BYTES:    "runtime.BytesSlice",
		descriptor.FieldDescriptorProto_TYPE_UINT32:   "runtime.Uint32Slice",
		descriptor.FieldDescriptorProto_TYPE_ENUM:     "runtime.EnumSlice",
		descriptor.FieldDescriptorProto_TYPE_SFIXED32: "runtime.Int32Slice",
		descriptor.FieldDescriptorProto_TYPE_SFIXED64: "runtime.Int64Slice",
		descriptor.FieldDescriptorProto_TYPE_SINT32:   "runtime.Int32Slice",
		descriptor.FieldDescriptorProto_TYPE_SINT64:   "runtime.Int64Slice",
	}

	proto2ConvertFuncs = map[descriptor.FieldDescriptorProto_Type]string{
		descriptor.FieldDescriptorProto_TYPE_DOUBLE:  "runtime.Float64P",
		descriptor.FieldDescriptorProto_TYPE_FLOAT:   "runtime.Float32P",
		descriptor.FieldDescriptorProto_TYPE_INT64:   "runtime.Int64P",
		descriptor.FieldDescriptorProto_TYPE_UINT64:  "runtime.Uint64P",
		descriptor.FieldDescriptorProto_TYPE_INT32:   "runtime.Int32P",
		descriptor.FieldDescriptorProto_TYPE_FIXED64: "runtime.Uint64P",
		descriptor.FieldDescriptorProto_TYPE_FIXED32: "runtime.Uint32P",
		descriptor.FieldDescriptorProto_TYPE_BOOL:    "runtime.BoolP",
		descriptor.FieldDescriptorProto_TYPE_STRING:  "runtime.StringP",
		// FieldDescriptorProto_TYPE_GROUP
		// FieldDescriptorProto_TYPE_MESSAGE
		// FieldDescriptorProto_TYPE_BYTES
		// TODO(yugui) Handle bytes
		descriptor.FieldDescriptorProto_TYPE_UINT32:   "runtime.Uint32P",
		descriptor.FieldDescriptorProto_TYPE_ENUM:     "runtime.EnumP",
		descriptor.FieldDescriptorProto_TYPE_SFIXED32: "runtime.Int32P",
		descriptor.FieldDescriptorProto_TYPE_SFIXED64: "runtime.Int64P",
		descriptor.FieldDescriptorProto_TYPE_SINT32:   "runtime.Int32P",
		descriptor.FieldDescriptorProto_TYPE_SINT64:   "runtime.Int64P",
	}

	proto2RepeatedConvertFuncs = map[descriptor.FieldDescriptorProto_Type]string{
		descriptor.FieldDescriptorProto_TYPE_DOUBLE:  "runtime.Float64Slice",
		descriptor.FieldDescriptorProto_TYPE_FLOAT:   "runtime.Float32Slice",
		descriptor.FieldDescriptorProto_TYPE_INT64:   "runtime.Int64Slice",
		descriptor.FieldDescriptorProto_TYPE_UINT64:  "runtime.Uint64Slice",
		descriptor.FieldDescriptorProto_TYPE_INT32:   "runtime.Int32Slice",
		descriptor.FieldDescriptorProto_TYPE_FIXED64: "runtime.Uint64Slice",
		descriptor.FieldDescriptorProto_TYPE_FIXED32: "runtime.Uint32Slice",
		descriptor.FieldDescriptorProto_TYPE_BOOL:    "runtime.BoolSlice",
		descriptor.FieldDescriptorProto_TYPE_STRING:  "runtime.StringSlice",
		// FieldDescriptorProto_TYPE_GROUP
		// FieldDescriptorProto_TYPE_MESSAGE
		// FieldDescriptorProto_TYPE_BYTES
		// TODO(maros7) Handle bytes
		descriptor.FieldDescriptorProto_TYPE_UINT32:   "runtime.Uint32Slice",
		descriptor.FieldDescriptorProto_TYPE_ENUM:     "runtime.EnumSlice",
		descriptor.FieldDescriptorProto_TYPE_SFIXED32: "runtime.Int32Slice",
		descriptor.FieldDescriptorProto_TYPE_SFIXED64: "runtime.Int64Slice",
		descriptor.FieldDescriptorProto_TYPE_SINT32:   "runtime.Int32Slice",
		descriptor.FieldDescriptorProto_TYPE_SINT64:   "runtime.Int64Slice",
	}
	wellKnownTypeConv = map[string]string{
		".google.protobuf.Timestamp":   "runtime.Timestamp",
		".google.protobuf.Duration":    "runtime.Duration",
		".google.protobuf.StringValue": "runtime.StringValue",
		".google.protobuf.FloatValue":  "runtime.FloatValue",
		".google.protobuf.DoubleValue": "runtime.DoubleValue",
		".google.protobuf.BoolValue":   "runtime.BoolValue",
		".google.protobuf.BytesValue":  "runtime.BytesValue",
		".google.protobuf.Int32Value":  "runtime.Int32Value",
		".google.protobuf.UInt32Value": "runtime.UInt32Value",
		".google.protobuf.Int64Value":  "runtime.Int64Value",
		".google.protobuf.UInt64Value": "runtime.UInt64Value",
	}
*/
)
