package compiler

import (
	"encoding/json"
	"fmt"
	"google.golang.org/protobuf/reflect/protoreflect"
	descriptor "google.golang.org/protobuf/types/descriptorpb"
	"reflect"
	"sort"
	"strings"
	"sync"
	tmpl "text/template"

	"github.com/Masterminds/sprig"
	"github.com/huandu/xstrings"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var ProtoHelpersFuncMap = tmpl.FuncMap{
	"string": func(i interface{}) string {
		return fmt.Sprintf("%v", i)
	},
	"type": func(v interface{}) string {
		return reflect.TypeOf(v).String()
	},
	"json": func(v interface{}) string {
		a, err := json.Marshal(v)
		if err != nil {
			return err.Error()
		}
		return string(a)
	},
	"prettyjson": func(v interface{}) string {
		a, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err.Error()
		}
		return string(a)
	},
	"splitArray": func(sep string, s string) []interface{} {
		var r []interface{}
		t := strings.Split(s, sep)
		for i := range t {
			if t[i] != "" {
				r = append(r, t[i])
			}
		}
		return r
	},
	"joinSort": func(sep string, s ...string) string {
		res := ""

		sort.Strings(s)
		for _, str := range s {
			if len(res) == 0 {
				res = fmt.Sprintf("%s", str)
			} else {
				res = fmt.Sprintf("%s%s%s", res, sep, str)
			}
		}
		return res
	},
	"first": func(a []string) string {
		return a[0]
	},
	"last": func(a []string) string {
		return a[len(a)-1]
	},
	"concat": func(a string, b ...string) string {
		return strings.Join(append([]string{a}, b...), "")
	},
	"join": func(sep string, a ...string) string {
		return strings.Join(a, sep)
	},
	"upperFirst": func(s string) string {
		return strings.ToUpper(s[:1]) + s[1:]
	},
	"lowerFirst": func(s string) string {
		return strings.ToLower(s[:1]) + s[1:]
	},
	"camelCase": func(s string) string {
		if len(s) > 1 {
			return xstrings.ToCamelCase(s)
		}

		return strings.ToUpper(s[:1])
	},
	"lowerCamelCase": func(s string) string {
		if len(s) > 1 {
			s = xstrings.ToCamelCase(s)
		}

		return strings.ToLower(s[:1]) + s[1:]
	},
	"upperCase": func(s string) string {
		return strings.ToUpper(s)
	},
	"kebabCase": func(s string) string {
		return strings.Replace(xstrings.ToSnakeCase(s), "_", "-", -1)
	},
	"contains": func(sub, s string) bool {
		return strings.Contains(s, sub)
	},
	"trimstr": func(cutset, s string) string {
		return strings.Trim(s, cutset)
	},
	"index": func(array interface{}, i int) interface{} {
		slice := reflect.ValueOf(array)
		if slice.Kind() != reflect.Slice {
			panic("Error in index(): given a non-slice type")
		}
		if i < 0 || i >= slice.Len() {
			panic("Error in index(): index out of bounds")
		}
		return slice.Index(i).Interface()
	},
	"add": func(a int, b int) int {
		return a + b
	},
	"subtract": func(a int, b int) int {
		return a - b
	},
	"multiply": func(a int, b int) int {
		return a * b
	},
	"divide": func(a int, b int) int {
		if b == 0 {
			panic("psssst ... little help here ... you cannot divide by 0")
		}
		return a / b
	},
	//"getProtoFile":               getProtoFile,
	//"getMessageType":             getMessageType,
	//"getEnumValue":               getEnumValue,
	//"isFieldMessage":             isFieldMessage,
	//"isFieldMessageTimeStamp":    isFieldMessageTimeStamp,
	//"isFieldRepeated":            isFieldRepeated,
	//"haskellType":                haskellType,
	//"goType":                     goType,
	//"goZeroValue":                goZeroValue,
	//"goTypeWithPackage":          goTypeWithPackage,
	//"snakeCase":                  xstrings.ToSnakeCase,
	"pragmaOnce":                   pragmaOnce,
	"goName":                       goName,
	"isHealthCheck":                isHealthCheck,
	"isNotHealthCheck":             isNotHealthCheck,
	"isPing":                       isPing,
	"isNotPing":                    isNotPing,
	"isAllMode":                    isAllMode,
	"isFileMode":                   isFileMode,
	"isServiceMode":                isServiceMode,
	"goTypeWithGoPackage":          goTypeWithGoPackage,
	"hasAnnotation":                hasAnnotation,
	"hasDirective":                 hasDirective,
	"shortType":                    shortType,
	"lowerGoNormalize":             lowerGoNormalize,
	"goNormalize":                  goNormalize,
	"stringFileOptionsExtension":   stringFileOptionsExtension,
	"stringMessageExtension":       stringMessageExtension,
	"stringFieldExtension":         stringFieldExtension,
	"int64FieldExtension":          int64FieldExtension,
	"int64MessageExtension":        int64MessageExtension,
	"stringMethodOptionsExtension": stringMethodOptionsExtension,
	"boolMethodOptionsExtension":   boolMethodOptionsExtension,
	"boolMessageExtension":         boolMessageExtension,
	"boolFieldExtension":           boolFieldExtension,
	"replaceDict":                  replaceDict,
	"setContext":                   setContext,
	"setStore":                     setStore,
	"getStore":                     getStore,
	"goPkg":                        goPkg,
	"goPkgLastElement":             goPkgLastElement,
}

var store = newStore()

// Utility to store some vars across multiple scope
type globalStore struct {
	store map[string]interface{}
	mu    sync.Mutex
}

func newStore() *globalStore {
	return &globalStore{
		store: make(map[string]interface{}),
	}
}

func (s *globalStore) getData(key string) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v, ok := s.store[key]; ok {
		return v
	}

	return false
}

func (s *globalStore) setData(key string, o interface{}) interface{} {
	s.mu.Lock()
	s.store[key] = o
	s.mu.Unlock()
	return o
}

func pragmaOnce(o *TemplateContext) bool {
	if d := getStore(o.RawFilename); d.(bool) {
		return false
	} else {
		setStore(o.RawFilename, true)
	}
	return true
}

func setContext(o *TemplateContext) *TemplateContext {
	key := fmt.Sprintf("%s_%s", "$this", o.File().Proto.GetName())
	store.setData(key, o)
	return o
}

func getContext(s string) *TemplateContext {
	key := fmt.Sprintf("%s_%s", "$this", s)
	return store.getData(key).(*TemplateContext)
}

func setStore(key string, o interface{}) string {
	store.setData(key, o)
	return ""
}

func getStore(key string) interface{} {
	return store.getData(key)
}

// stringMethodOptionsExtension extracts method options of a string type.
// To define your own extensions see:
// https://developers.google.com/protocol-buffers/docs/proto#customoptions
// Typically the fieldID of private extensions should be in the range:
// 50000-99999
func stringMethodOptionsExtension(fieldID int32, f *descriptor.MethodDescriptorProto) string {
	if f == nil {
		return ""
	}
	if f.Options == nil {
		return ""
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return ""
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return ""
	}

	str, ok := ext.(*string)
	if !ok {
		return ""
	}

	return *str
}

// stringFileOptionsExtension extracts file options of a string type.
// To define your own extensions see:
// https://developers.google.com/protocol-buffers/docs/proto#customoptions
// Typically the fieldID of private extensions should be in the range:
// 50000-99999
func stringFileOptionsExtension(fieldID int32, f *descriptor.FileDescriptorProto) string {
	if f == nil {
		return ""
	}
	if f.Options == nil {
		return ""
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return ""
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return ""
	}

	str, ok := ext.(*string)
	if !ok {
		return ""
	}

	return *str
}

func stringFieldExtension(fieldID int32, f *descriptor.FieldDescriptorProto) string {
	if f == nil {
		return ""
	}
	if f.Options == nil {
		return ""
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return ""
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return ""
	}

	str, ok := ext.(*string)
	if !ok {
		return ""
	}

	return *str
}

func int64FieldExtension(fieldID int32, f *descriptor.FieldDescriptorProto) int64 {
	if f == nil {
		return 0
	}
	if f.Options == nil {
		return 0
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return 0
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return 0
	}

	i, ok := ext.(*int64)
	if !ok {
		return 0
	}

	return *i
}

func int64MessageExtension(fieldID int32, f *descriptor.DescriptorProto) int64 {
	if f == nil {
		return 0
	}
	if f.Options == nil {
		return 0
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return 0
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return 0
	}

	i, ok := ext.(*int64)
	if !ok {
		return 0
	}

	return *i
}

func stringMessageExtension(fieldID int32, f *descriptor.DescriptorProto) string {
	if f == nil {
		return ""
	}
	if f.Options == nil {
		return ""
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return ""
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return ""
	}

	str, ok := ext.(*string)
	if !ok {
		return ""
	}

	return *str
}

func boolMethodOptionsExtension(fieldID int32, f *descriptor.MethodDescriptorProto) bool {
	if f == nil {
		return false
	}
	if f.Options == nil {
		return false
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return false
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return false
	}

	b, ok := ext.(*bool)
	if !ok {
		return false
	}

	return *b
}

func boolFieldExtension(fieldID int32, f *descriptor.FieldDescriptorProto) bool {
	if f == nil {
		return false
	}
	if f.Options == nil {
		return false
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return false
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return false
	}

	b, ok := ext.(*bool)
	if !ok {
		return false
	}

	return *b
}

func boolMessageExtension(fieldID int32, f *descriptor.DescriptorProto) bool {
	if f == nil {
		return false
	}
	if f.Options == nil {
		return false
	}

	eType, err := protoregistry.GlobalTypes.FindExtensionByNumber(proto.MessageName(f.Options), protoreflect.FieldNumber(fieldID))
	if eType == nil && err == protoregistry.NotFound {
		return false
	}

	ext := proto.GetExtension(f.Options, eType)
	if err != nil {
		return false
	}

	b, ok := ext.(*bool)
	if !ok {
		return false
	}

	return *b
}

func init() {
	for k, v := range sprig.TxtFuncMap() {
		ProtoHelpersFuncMap[k] = v
	}
}

func goPackage(context interface{}) string {
	switch context.(type) {
	case *File:
		return string(context.(*File).GoPackageName)
	case *Service:
		return string(context.(*Service).File.GoPackageName)
	}
	return ""
}

func goName(context interface{}) string {
	switch context.(type) {
	case *File:
		return context.(*File).GoDescriptorIdent.GoName
	case *Service:
		return context.(*Service).GoName
	}
	return ""
}

func isAllMode(mode Mode) bool {
	return mode == AllMode
}

func isFileMode(mode Mode) bool {
	return mode == FileMode
}

func isServiceMode(mode Mode) bool {
	return mode == ServiceMode
}

func hasAnnotation(c CommentSet, str string) bool {
	if !strings.HasPrefix(str, "@") {
		str = "@" + str
	}
	return commentPrefix(c, str)
}

func hasDirective(c CommentSet, str string) bool {
	str = strings.ToLower(str)
	for _, d := range c.Directives {
		dirStr := strings.ToLower(strings.Trim(d.Directive, " \t\r\n"))
		if strings.HasPrefix(dirStr, str) {
			return true
		}
	}

	return false
}

func commentPrefix(c CommentSet, str string) bool {
	checkComment := func(comments ...Comments) bool {
		for _, comment := range comments {
			if len(string(comment)) > 0 && strings.HasPrefix(strings.ToLower(strings.Trim(string(comment), " \t\r\n")), str) {
				return true
			}
		}
		return false
	}

	return checkComment(c.Leading) || checkComment(c.Trailing) || checkComment(c.LeadingDetached...)
}

func isNotHealthCheck(m *Method) bool {
	return !hasAnnotation(m.Comments, "@@healthcheck")
}

func isHealthCheck(m *Method) bool {
	return hasAnnotation(m.Comments, "@@healthcheck")
}

func isNotPing(m *Method) bool {
	return !hasAnnotation(m.Comments, "@@ping")
}

func isPing(m *Method) bool {
	return hasAnnotation(m.Comments, "@@ping")
}

func goMethodType(tc *TemplateContext, m *Method) string {
	return renderMethodWithArgs(tc, m)
}

func goMethodDefinition(tc *TemplateContext, m *Method, parameterNames ...string) string {
	defs := []string{"ctx", "in", "out"}
	if parameterNames == nil {
		parameterNames = defs
	} else if len(parameterNames) < 3 {
		parameterNames = append(parameterNames, defs[len(parameterNames):]...)
	}
	return renderMethodWithArgs(tc, m, parameterNames...)
}

func renderMethodWithArgs(tc *TemplateContext, m *Method, p ...string) string {
	args, prefix, sep := "", "", ""
	if p == nil {
		p = []string{"", "", ""}
	} else {
		sep = " "
	}
	inPkg := string(tc.file.GoPackageName)
	outPkg := string(tc.file.GoPackageName)
	inPkg = fmt.Sprintf("%s.", tc.AddImport(string(m.Input.GoIdent.GoImportPath)))
	outPkg = fmt.Sprintf("%s.", tc.AddImport(string(m.Output.GoIdent.GoImportPath)))
	ctx := fmt.Sprintf("%s%s%s.%s", p[0], sep, "context", "Context")
	inType := fmt.Sprintf("%s%s*%s%s", p[1], sep, inPkg, m.Input.GoIdent.GoName)
	outType := fmt.Sprintf("%s%s*%s%s", p[2], sep, outPkg, m.Output.GoIdent.GoName)
	if m.Desc.IsStreamingClient() {
		args = fmt.Sprintf("%s, %s%s%s%s_%sStream", ctx, p[1]+p[2], sep, inPkg, m.Parent.GoName, m.GoName)
	} else {
		if m.Desc.IsStreamingServer() {
			outType = fmt.Sprintf("%s%s%s%s_%sStream", p[2], sep, outPkg, m.Parent.GoName, m.GoName)
		}
		args = fmt.Sprintf("%s, %s, %s", ctx, inType, outType)
	}
	return fmt.Sprintf("%s%s(%s) error", prefix, m.GoName, args)
}

// goTypeWithGoPackage types the field MESSAGE and ENUM with the go_package name.
// This method is an evolution of goTypeWithPackage. It handles message embedded.
//
// example:
// ```protoregistry
// message GetArticleResponse {
// 	Article article = 1;
// 	message Storage {
// 		  string code = 1;
// 	}
// 	repeated Storage storages = 2;
// }
// ```
// Then the type of `storages` is `GetArticleResponse_Storage` for the go language.
//
func goTypeWithGoPackage(p *File, f *Field) string {
	pkg := ""
	if f.Desc.Kind() == protoreflect.MessageKind || f.Desc.Kind() == protoreflect.EnumKind {
		if isTimestampPackage(string(f.Desc.FullName())) {
			pkg = "timestamp"
		} else {
			pkg = string(p.GoPackageName)
			if strings.Contains(pkg, ";") {
				pkg = strings.Split(pkg, ";")[1]
			}
		}
	}
	return goTypeWithEmbedded(pkg, f, p)
}

func goTypeWithEmbedded(pkg string, f *Field, p *File) string {
	if pkg != "" {
		pkg = pkg + "."
	}
	switch f.Desc.Kind() {
	case protoreflect.DoubleKind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]float64"
		}
		return "float64"
	case protoreflect.FloatKind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]float32"
		}
		return "float32"
	case protoreflect.Int64Kind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]int64"
		}
		return "int64"
	case protoreflect.Uint64Kind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]uint64"
		}
		return "uint64"
	case protoreflect.Int32Kind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]int32"
		}
		return "int32"
	case protoreflect.Uint32Kind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]uint32"
		}
		return "uint32"
	case protoreflect.BoolKind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]bool"
		}
		return "bool"
	case protoreflect.StringKind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]string"
		}
		return "string"
	case protoreflect.MessageKind:
		name := f.Message.GoIdent.GoName
		if f.Desc.Cardinality() == protoreflect.Repeated {
			fieldPackage := strings.Split(f.Message.GoIdent.GoName, ".")
			filePackage := strings.Split(string(p.Desc.Package()), ".")
			// check if we are working with a message embedded.
			if len(fieldPackage) > 1 && len(fieldPackage)+1 > len(filePackage) {
				name = strings.Join(fieldPackage[len(filePackage):], "_")
			}

			return fmt.Sprintf("[]*%s%s", pkg, shortType(name))
		}
		return fmt.Sprintf("*%s%s", pkg, shortType(name))
	case protoreflect.BytesKind:
		if f.Desc.Cardinality() == protoreflect.Repeated {
			return "[]byte"
		}
		return "byte"
	case protoreflect.EnumKind:
		name := f.Message.GoIdent.GoName
		fieldPackage := strings.Split(f.Message.GoIdent.GoName, ".")
		filePackage := strings.Split(string(p.Desc.Package()), ".")
		// check if we are working with a message embedded.
		if len(fieldPackage) > 1 && len(fieldPackage)+1 > len(filePackage) {
			name = strings.Join(fieldPackage[len(filePackage):], "_")
		}
		return fmt.Sprintf("*%s%s", pkg, shortType(name))
	default:
		return "interface{}"
	}
}

func isTimestampPackage(s string) bool {
	var isTimestampPackage bool
	if strings.Compare(s, ".google.protobuf.Timestamp") == 0 {
		isTimestampPackage = true
	}
	return isTimestampPackage
}

func getPackageTypeName(s string) string {
	if strings.Contains(s, ".") {
		return strings.Split(s, ".")[1]
	}
	return ""
}

func shortType(s string) string {
	t := strings.Split(s, ".")
	return t[len(t)-1]
}

// lowerGoNormalize takes a string and applies formatting
// rules to conform to Golang convention. It applies a camel
// case filter, lowers the first character and formats fields
// with `id` to `ID`.
func lowerGoNormalize(s string) string {
	fmtd := xstrings.ToCamelCase(s)
	fmtd = xstrings.FirstRuneToLower(fmtd)
	return formatID(s, fmtd)
}

// goNormalize takes a string and applies formatting rules
// to conform to Golang convention. It applies a camel case
// filter and formats fields with `id` to `ID`.
func goNormalize(s string) string {
	fmtd := xstrings.ToCamelCase(s)
	return formatID(s, fmtd)
}

// formatID takes a base string alonsgide a formatted string.
// It acts as a transformation filter for fields containing
// `id` in order to conform to Golang convention.
func formatID(base string, formatted string) string {
	if formatted == "" {
		return formatted
	}
	switch {
	case base == "id":
		// id -> ID
		return "ID"
	case strings.HasPrefix(base, "id_"):
		// id_some -> IDSome
		return "ID" + formatted[2:]
	case strings.HasSuffix(base, "_id"):
		// some_id -> SomeID
		return formatted[:len(formatted)-2] + "ID"
	case strings.HasSuffix(base, "_ids"):
		// some_ids -> SomeIDs
		return formatted[:len(formatted)-3] + "IDs"
	}
	return formatted
}

func replaceDict(src string, dict map[string]interface{}) string {
	for old, v := range dict {
		nval, ok := v.(string)
		if !ok {
			continue
		}
		src = strings.Replace(src, old, nval, -1)
	}
	return src
}

func goPkg(f *File) string {
	return string(f.GoPackageName)
}

func goPkgLastElement(f *File) string {
	pkg := goPkg(f)
	pkgSplitted := strings.Split(pkg, "/")
	return pkgSplitted[len(pkgSplitted)-1]
}

var isGoPredeclaredIdentifier = map[string]bool{
	"append":     true,
	"bool":       true,
	"byte":       true,
	"cap":        true,
	"close":      true,
	"complex":    true,
	"complex128": true,
	"complex64":  true,
	"copy":       true,
	"delete":     true,
	"error":      true,
	"false":      true,
	"float32":    true,
	"float64":    true,
	"imag":       true,
	"int":        true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"int8":       true,
	"iota":       true,
	"len":        true,
	"make":       true,
	"new":        true,
	"nil":        true,
	"panic":      true,
	"print":      true,
	"println":    true,
	"real":       true,
	"recover":    true,
	"rune":       true,
	"string":     true,
	"true":       true,
	"uint":       true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uint8":      true,
	"uintptr":    true,
}
