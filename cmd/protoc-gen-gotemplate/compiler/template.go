// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compiler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/chrismoran-blockfi/protoc-gen-gotemplate/internal/genid"
	"github.com/chrismoran-blockfi/protoc-gen-gotemplate/internal/strs"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A Plugin is a protoc plugin invocation.
type Plugin struct {
	// Request is the CodeGeneratorRequest provided by protoc.
	Request *pluginpb.CodeGeneratorRequest

	// Files is the set of files to generate and everything they import.
	// Files appear in topological order, so each file appears before any
	// file that imports it.
	Files       []*File
	FilesByPath map[string]*File

	// SupportedFeatures is the set of protobuf language features supported by
	// this generator plugin. See the documentation for
	// google.protobuf.CodeGeneratorResponse.supported_features for details.
	SupportedFeatures uint64

	fileReg        *protoregistry.Files
	enumsByName    map[protoreflect.FullName]*Enum
	messagesByName map[protoreflect.FullName]*Message
	templateDir    string
	destinationDir string
	mode           Mode
	index          int
	debug          bool
	genFiles       []*GeneratedFile
	opts           Options
	err            error
}

type Options struct {
	ParamFunc func(name, value string) error

	// ImportRewriteFunc is called with the import path of each package
	// imported by a generated file. It returns the import path to use
	// for this package.
	ImportRewriteFunc func(GoImportPath) GoImportPath
}

// Run executes a function as a protoc plugin.
//
// It reads a CodeGeneratorRequest message from os.Stdin, invokes the plugin
// function, and writes a CodeGeneratorResponse message to os.Stdout.
//
// If a failure occurs while reading or writing, Run prints an error to
// os.Stderr and calls os.Exit(1).
func (opts Options) Run(f func(*Plugin) error) {
	if err := run(opts, f); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
}

func run(opts Options, f func(*Plugin) error) error {
	if len(os.Args) > 1 {
		return fmt.Errorf("unknown argument %q (this program should be run by protoc, not directly)", os.Args[1])
	}
	in, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err = proto.Unmarshal(in, req); err != nil {
		return err
	}
	sort.Strings(req.FileToGenerate)
	gen, err := opts.New(req)
	if err != nil {
		return err
	}
	if err = f(gen); err != nil {
		// Errors from the plugin function are reported by setting the
		// error field in the CodeGeneratorResponse.
		//
		// In contrast, errors that indicate a problem in protoc
		// itself (unparsable input, I/O errors, etc.) are reported
		// to stderr.
		gen.Error(err)
	}
	resp := gen.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	if doDebug, ok := os.LookupEnv("PROTOC_GEN_GOTEMPLATE_DEBUG"); ok && len(doDebug) > 0 {
		jsonOut, _ := json.MarshalIndent(resp, "", "  ")
		_ = ioutil.WriteFile("resp.json", jsonOut, 0644)
	} else {
		if _, err = os.Stdout.Write(out); err != nil {
			return err
		}
	}
	return nil
}

func allTemplates(plug *Plugin, directable Directable) ([]*template, error) {
	templates := make([]*template, 0)
	err := filepath.Walk(plug.templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".tmpl" {
			return nil
		}
		rel, err := filepath.Rel(plug.templateDir, path)
		if err != nil {
			return err
		}
		if plug.debug {
			log.Printf("new template: %q", rel)
		}

		templates = append(
			templates,
			newTemplate(
				withRawFilename(rel),
				withKind(fileTemplateKind),
				withTemplateDir(plug.templateDir),
			),
		)

		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, dir := range directable.Directives() {
		if dir.Directive == "protoc_insert" {
			if plug.debug {
				log.Printf("new directive template from:%s %s@%s", dir.Source, dir.Params[0], dir.Params[1])
			}
			templates = append(
				templates,
				newTemplate(
					withRawFilename(dir.Params[0]),
					withKind(directiveTemplateKind),
					withContent(dir.Value),
					withSource(dir.Source),
					withTemplateDir(plug.templateDir),
					withInsertionPoint(dir.Params[1]),
				),
			)
		}
	}
	sort.Sort(templateSorter(templates))
	return templates, nil
}

func ProcessTemplates(gen *Plugin) {
	directableArray := make([]Directable, 0, len(gen.Files))
	for _, f := range gen.Files {
		switch gen.mode {
		case AllMode:
			directableArray = append(directableArray, f)
		case ServiceMode:
			for _, s := range f.Services {
				directableArray = append(directableArray, s)
			}
		case FileMode:
			if s := f.Services; s != nil && len(s) > 0 {
				directableArray = append(directableArray, f)
			}
		}
	}

	for index, dir := range directableArray {
		templates, err := allTemplates(gen, dir)
		if err != nil {
			gen.Error(err)
			return
		}
		templatesLen := len(templates)
		errChan := make(chan error, templatesLen)
		resultChan := make(chan *GeneratedFile, templatesLen)
		for _, itemplate := range templates {
			go func(t *template) {
				cindex := gen.index
				if cindex == -1 {
					cindex = index
				}

				var templateContext *TemplateContext
				if templateContext, err = t.executeTemplate(gen.mode, gen.destinationDir, cindex, dir); err != nil {
					errChan <- err
					return
				}

				gf := gen.NewGeneratedFile(t, templateContext)

				resultChan <- gf
			}(itemplate)
		}
		for i := 0; i < templatesLen; i++ {
			select {
			case f := <-resultChan:
				gen.genFiles = append(gen.genFiles, f)
			case err = <-errChan:
				panic(err)
			}
		}
	}
	sort.Sort(genFileSorter(gen.genFiles))

	for i, curFile := range gen.genFiles {
		if curFile.skip {
			continue
		}
		for _, icurFile := range gen.genFiles[i+1:] {
			if icurFile.skip {
				continue
			}
			if curFile.fileName == icurFile.fileName && curFile.insertionPoint == icurFile.insertionPoint {
				if len(icurFile.insertionPoint) > 0 && len(strings.Trim(string(icurFile.buf.Bytes()), " \r\n\t")) > 0 {
					curFile.buf.Write(icurFile.buf.Bytes())
				}
				icurFile.Skip()
			}
		}
	}
}

// New returns a new Plugin.
func (opts Options) New(req *pluginpb.CodeGeneratorRequest) (*Plugin, error) {
	gen := &Plugin{
		Request:        req,
		FilesByPath:    make(map[string]*File),
		fileReg:        new(protoregistry.Files),
		enumsByName:    make(map[protoreflect.FullName]*Enum),
		messagesByName: make(map[protoreflect.FullName]*Message),
		opts:           opts,
		index:          -1,
		templateDir:    "./templates",
		destinationDir: ".",
		mode:           AllMode,
		debug:          false,
	}

	packageNames := make(map[string]GoPackageName) // filename -> package name
	importPaths := make(map[string]GoImportPath)   // filename -> import path
	for _, param := range strings.Split(req.GetParameter(), ",") {
		var value string
		if i := strings.Index(param, "="); i >= 0 {
			value = param[i+1:]
			param = param[0:i]
		}
		switch param {
		case "":
			// Ignore.
		case "template_dir":
			gen.templateDir = value
		case "destination_dir":
			gen.destinationDir = value
		case "debug":
			switch strings.ToLower(value) {
			case boolTrue, "t":
				gen.debug = true
			case boolFalse, "f":
			default:
				log.Printf("Err: invalid value for debug: %q", value)
			}
		case "index":
			if index, err := strconv.Atoi(value); err != nil {
				return nil, err
			} else {
				gen.index = index
			}
		case "mode":
			var modeFlag Mode
			switch value {
			case "all":
				modeFlag = AllMode
			case "file":
				modeFlag = FileMode
			case "service":
				modeFlag = ServiceMode
			default:
				modeFlag = ServiceMode
			}
			gen.mode = modeFlag
		default:
			if opts.ParamFunc != nil {
				if err := opts.ParamFunc(param, value); err != nil {
					return nil, err
				}
			}
		}
	}

	// Figure out the import path and package name for each file.
	//
	// The rules here are complicated and have grown organically over time.
	// Interactions between different ways of specifying package information
	// may be surprising.
	//
	// The recommended approach is to include a go_package option in every
	// .proto source file specifying the full import path of the Go package
	// associated with this file.
	//
	//     option go_package = "google.golang.org/protobuf/types/known/anypb";
	//
	// Alternatively, build systems which want to exert full control over
	// import paths may specify M<filename>=<import_path> flags.
	for _, fdesc := range gen.Request.ProtoFile {
		// The "M" command-line flags take precedence over
		// the "go_package" option in the .proto source file.
		filename := fdesc.GetName()
		impPath, pkgName := splitImportPathAndPackageName(fdesc.GetOptions().GetGoPackage())
		if importPaths[filename] == "" && impPath != "" {
			importPaths[filename] = impPath
		}
		if packageNames[filename] == "" && pkgName != "" {
			packageNames[filename] = pkgName
		}
		switch {
		case importPaths[filename] == "":
			// The import path must be specified one way or another.
			return nil, fmt.Errorf(
				"unable to determine Go import path for %q\n\n"+
					"Please specify either:\n"+
					"\t• a \"go_package\" option in the .proto source file, or\n"+
					"\t• a \"M\" argument on the command line.\n\n"+
					"See %v for more information.\n",
				fdesc.GetName(), goPackageDocURL)
		case !strings.Contains(string(importPaths[filename]), ".") &&
			!strings.Contains(string(importPaths[filename]), "/"):
			// Check that import paths contain at least a dot or slash to avoid
			// a common mistake where import path is confused with package name.
			return nil, fmt.Errorf(
				"invalid Go import path %q for %q\n\n"+
					"The import path must contain at least one period ('.') or forward slash ('/') character.\n\n"+
					"See %v for more information.\n",
				string(importPaths[filename]), fdesc.GetName(), goPackageDocURL)
		case packageNames[filename] == "":
			// If the package name is not explicitly specified,
			// then derive a reasonable package name from the import path.
			//
			// NOTE: The package name is derived first from the import path in
			// the "go_package" option (if present) before trying the "M" flag.
			// The inverted order for this is because the primary use of the "M"
			// flag is by build systems that have full control over the
			// import paths all packages, where it is generally expected that
			// the Go package name still be identical for the Go toolchain and
			// for custom build systems like Bazel.
			if impPath == "" {
				impPath = importPaths[filename]
			}
			packageNames[filename] = cleanPackageName(path.Base(string(impPath)))
		}
	}

	// Consistency check: Every file with the same Go import path should have
	// the same Go package name.
	packageFiles := make(map[GoImportPath][]string)
	for filename, importPath := range importPaths {
		if _, ok := packageNames[filename]; !ok {
			// Skip files mentioned in a M<file>=<import_path> parameter
			// but which do not appear in the CodeGeneratorRequest.
			continue
		}
		packageFiles[importPath] = append(packageFiles[importPath], filename)
	}
	for importPath, filenames := range packageFiles {
		for i := 1; i < len(filenames); i++ {
			if a, b := packageNames[filenames[0]], packageNames[filenames[i]]; a != b {
				return nil, fmt.Errorf("go package %v has inconsistent names %v (%v) and %v (%v)",
					importPath, a, filenames[0], b, filenames[i])
			}
		}
	}

	for _, fdesc := range gen.Request.ProtoFile {
		filename := fdesc.GetName()
		if gen.FilesByPath[filename] != nil {
			return nil, fmt.Errorf("duplicate file name: %q", filename)
		}
		f, err := newFile(gen, fdesc, packageNames[filename], importPaths[filename])
		if err != nil {
			return nil, err
		}
		gen.Files = append(gen.Files, f)
		gen.FilesByPath[filename] = f
	}
	for _, filename := range gen.Request.FileToGenerate {
		f, ok := gen.FilesByPath[filename]
		if !ok {
			return nil, fmt.Errorf("no descriptor for generated file: %v", filename)
		}
		f.Generate = true
	}
	return gen, nil
}

// Error records an error in code generation. The generator will report the
// error back to protoc and will not produce output.
func (gen *Plugin) Error(err error) {
	if gen.err == nil {
		gen.err = err
	}
}

// Response returns the generator output.
func (gen *Plugin) Response() *pluginpb.CodeGeneratorResponse {
	resp := &pluginpb.CodeGeneratorResponse{}
	if gen.err != nil {
		resp.Error = proto.String(gen.err.Error())
		return resp
	}

	for _, g := range gen.genFiles {
		if g.skip {
			continue
		}
		content, err := g.Content()
		if err != nil {
			return &pluginpb.CodeGeneratorResponse{
				Error: proto.String(err.Error()),
			}
		}
		filename := g.fileName
		insertionPoint := g.insertionPoint
		if len(insertionPoint) > 0 {
			resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
				Name:           proto.String(filename),
				Content:        proto.String(string(content)),
				InsertionPoint: proto.String(insertionPoint),
			})
		} else {
			resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
				Name:    proto.String(filename),
				Content: proto.String(string(content)),
			})
		}
	}
	sort.Sort(ResponseSorter(resp.File))
	if gen.SupportedFeatures > 0 {
		resp.SupportedFeatures = proto.Uint64(gen.SupportedFeatures)
	}
	return resp
}

// A File describes a .proto source file.
type File struct {
	Desc  protoreflect.FileDescriptor
	Proto *descriptorpb.FileDescriptorProto

	GoDescriptorIdent GoIdent       // name of Go variable for the file descriptor
	GoPackageName     GoPackageName // name of this file's Go package
	GoImportPath      GoImportPath  // import path of this file's Go package

	Enums      []*Enum      // top-level enum declarations
	Messages   []*Message   // top-level message declarations
	Extensions []*Extension // top-level extension declarations
	Services   []*Service   // top-level service declarations

	directives []CommentDirective

	Generate bool // true if we should generate code for this file

	// GeneratedFilenamePrefix is used to construct filenames for generated
	// files associated with this source file.
	//
	// For example, the source file "dir/foo.proto" might have a filename prefix
	// of "dir/foo". Appending ".pb.go" produces an output file of "dir/foo.pb.go".
	GeneratedFilenamePrefix string

	location Location
}

func (f *File) Directives() []CommentDirective {
	return f.directives
}

func newFile(gen *Plugin, p *descriptorpb.FileDescriptorProto, packageName GoPackageName, importPath GoImportPath) (*File, error) {
	var desc protoreflect.FileDescriptor
	var err error

	if desc, err = protodesc.NewFile(p, gen.fileReg); err != nil {
		return nil, fmt.Errorf("invalid FileDescriptorProto %q: %v", p.GetName(), err)
	}
	if err = gen.fileReg.RegisterFile(desc); err != nil {
		return nil, fmt.Errorf("cannot register descriptor %q: %v", p.GetName(), err)
	}
	f := &File{
		Desc:          desc,
		Proto:         p,
		GoPackageName: packageName,
		GoImportPath:  importPath,
		location:      Location{SourceFile: desc.Path()},
		directives:    make([]CommentDirective, 0),
	}

	// Determine the prefix for generated Go files.
	prefix := p.GetName()
	if ext := path.Ext(prefix); ext == ".proto" || ext == ".protodevel" {
		prefix = prefix[:len(prefix)-len(ext)]
	}
	f.GoDescriptorIdent = GoIdent{
		GoName:       strs.GoSanitized(p.GetName()),
		GoImportPath: f.GoImportPath,
	}
	f.GeneratedFilenamePrefix = prefix

	for i, eds := 0, desc.Enums(); i < eds.Len(); i++ {
		f.Enums = append(f.Enums, newEnum(gen, f, nil, eds.Get(i)))
	}
	for i, mds := 0, desc.Messages(); i < mds.Len(); i++ {
		f.Messages = append(f.Messages, newMessage(gen, f, nil, mds.Get(i)))
	}
	for i, xds := 0, desc.Extensions(); i < xds.Len(); i++ {
		f.Extensions = append(f.Extensions, newField(gen, f, nil, xds.Get(i)))
	}
	for i, sds := 0, desc.Services(); i < sds.Len(); i++ {
		f.Services = append(f.Services, newService(gen, f, sds.Get(i)))
	}
	for _, message := range f.Messages {
		if err = message.resolveDependencies(gen); err != nil {
			return nil, err
		}
	}
	for _, extension := range f.Extensions {
		if err = extension.resolveDependencies(gen); err != nil {
			return nil, err
		}
	}
	for _, service := range f.Services {
		for _, method := range service.Methods {
			if err = method.resolveDependencies(gen); err != nil {
				return nil, err
			}
		}
	}
	return f, nil
}

// splitImportPathAndPackageName splits off the optional Go package name
// from the Go import path when seperated by a ';' delimiter.
func splitImportPathAndPackageName(s string) (GoImportPath, GoPackageName) {
	if i := strings.Index(s, ";"); i >= 0 {
		return GoImportPath(s[:i]), GoPackageName(s[i+1:])
	}
	return GoImportPath(s), ""
}

// An Enum describes an enum.
type Enum struct {
	Desc protoreflect.EnumDescriptor

	File    *File
	GoIdent GoIdent // name of the generated Go type

	Values []*EnumValue // enum value declarations

	Location Location   // location of this enum
	Comments CommentSet // comments associated with this enum
}

func newEnum(gen *Plugin, f *File, parent *Message, desc protoreflect.EnumDescriptor) *Enum {
	var loc Location
	if parent != nil {
		loc = parent.Location.appendPath(genid.DescriptorProto_EnumType_field_number, desc.Index())
	} else {
		loc = f.location.appendPath(genid.FileDescriptorProto_EnumType_field_number, desc.Index())
	}
	enum := &Enum{
		Desc:     desc,
		File:     f,
		GoIdent:  newGoIdent(f, desc),
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	parseDirectives(&enum.Comments, enum, enum.Desc.ParentFile().Path())
	f.directives = append(f.directives, enum.Directives()...)
	gen.enumsByName[desc.FullName()] = enum
	for i, vds := 0, enum.Desc.Values(); i < vds.Len(); i++ {
		enum.Values = append(enum.Values, newEnumValue(gen, f, parent, enum, vds.Get(i)))
	}
	return enum
}

func (e *Enum) Directives() []CommentDirective {
	return e.Comments.Directives
}

// An EnumValue describes an enum value.
type EnumValue struct {
	Desc protoreflect.EnumValueDescriptor

	GoIdent GoIdent // name of the generated Go declaration

	Parent *Enum // enum in which this value is declared

	Location Location   // location of this enum value
	Comments CommentSet // comments associated with this enum value
}

func newEnumValue(_ *Plugin, f *File, message *Message, enum *Enum, desc protoreflect.EnumValueDescriptor) *EnumValue {
	// A top-level enum value's name is: EnumName_ValueName
	// An enum value contained in a message is: MessageName_ValueName
	//
	// For historical reasons, enum value names are not camel-cased.
	parentIdent := enum.GoIdent
	if message != nil {
		parentIdent = message.GoIdent
	}
	name := parentIdent.GoName + "_" + string(desc.Name())
	loc := enum.Location.appendPath(genid.EnumDescriptorProto_Value_field_number, desc.Index())
	enumValue := &EnumValue{
		Desc:     desc,
		GoIdent:  f.GoImportPath.Ident(name),
		Parent:   enum,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	parseDirectives(&enumValue.Comments, enumValue, enumValue.Desc.ParentFile().Path())

	return enumValue
}

func (e *EnumValue) Directives() []CommentDirective {
	return e.Comments.Directives
}

// A Message describes a message.
type Message struct {
	Desc protoreflect.MessageDescriptor

	GoIdent GoIdent // name of the generated Go type

	Fields []*Field // message field declarations
	Oneofs []*Oneof // message oneof declarations

	Enums      []*Enum      // nested enum declarations
	Messages   []*Message   // nested message declarations
	Extensions []*Extension // nested extension declarations

	Location Location   // location of this message
	Comments CommentSet // comments associated with this message
}

func newMessage(gen *Plugin, f *File, parent *Message, desc protoreflect.MessageDescriptor) *Message {
	var loc Location
	if parent != nil {
		loc = parent.Location.appendPath(genid.DescriptorProto_NestedType_field_number, desc.Index())
	} else {
		loc = f.location.appendPath(genid.FileDescriptorProto_MessageType_field_number, desc.Index())
	}
	message := &Message{
		Desc:     desc,
		GoIdent:  newGoIdent(f, desc),
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	parseDirectives(&message.Comments, message, message.Desc.ParentFile().Path())
	f.directives = append(f.directives, message.Directives()...)
	gen.messagesByName[desc.FullName()] = message
	for i, eds := 0, desc.Enums(); i < eds.Len(); i++ {
		message.Enums = append(message.Enums, newEnum(gen, f, message, eds.Get(i)))
	}
	for i, mds := 0, desc.Messages(); i < mds.Len(); i++ {
		message.Messages = append(message.Messages, newMessage(gen, f, message, mds.Get(i)))
	}
	for i, fds := 0, desc.Fields(); i < fds.Len(); i++ {
		message.Fields = append(message.Fields, newField(gen, f, message, fds.Get(i)))
	}
	for i, ods := 0, desc.Oneofs(); i < ods.Len(); i++ {
		message.Oneofs = append(message.Oneofs, newOneof(gen, f, message, ods.Get(i)))
	}
	for i, xds := 0, desc.Extensions(); i < xds.Len(); i++ {
		message.Extensions = append(message.Extensions, newField(gen, f, message, xds.Get(i)))
	}

	// Resolve local references between fields and oneofs.
	for _, field := range message.Fields {
		if od := field.Desc.ContainingOneof(); od != nil {
			oneof := message.Oneofs[od.Index()]
			field.Oneof = oneof
			oneof.Fields = append(oneof.Fields, field)
		}
	}

	// Field name conflict resolution.
	//
	// We assume well-known method names that may be attached to a generated
	// message type, as well as a 'Get*' method for each field. For each
	// field in turn, we add _s to its name until there are no conflicts.
	//
	// Any change to the following set of method names is a potential
	// incompatible API change because it may change generated field names.
	//
	// TODO: If we ever support a 'go_name' option to set the Go name of a
	// field, we should consider dropping this entirely. The conflict
	// resolution algorithm is subtle and surprising (changing the order
	// in which fields appear in the .proto source file can change the
	// names of fields in generated code), and does not adapt well to
	// adding new per-field methods such as setters.
	usedNames := map[string]bool{
		"Reset":               true,
		"String":              true,
		"ProtoMessage":        true,
		"Marshal":             true,
		"Unmarshal":           true,
		"ExtensionRangeArray": true,
		"ExtensionMap":        true,
		"Descriptor":          true,
	}
	makeNameUnique := func(name string, hasGetter bool) string {
		for usedNames[name] || (hasGetter && usedNames["Get"+name]) {
			name += "_"
		}
		usedNames[name] = true
		usedNames["Get"+name] = hasGetter
		return name
	}
	for _, field := range message.Fields {
		field.GoName = makeNameUnique(field.GoName, true)
		field.GoIdent.GoName = message.GoIdent.GoName + "_" + field.GoName
		if field.Oneof != nil && field.Oneof.Fields[0] == field {
			// Make the name for a oneof unique as well. For historical reasons,
			// this assumes that a getter method is not generated for oneofs.
			// This is incorrect, but fixing it breaks existing code.
			field.Oneof.GoName = makeNameUnique(field.Oneof.GoName, false)
			field.Oneof.GoIdent.GoName = message.GoIdent.GoName + "_" + field.Oneof.GoName
		}
	}

	// Oneof field name conflict resolution.
	//
	// This conflict resolution is incomplete as it does not consider collisions
	// with other oneof field types, but fixing it breaks existing code.
	for _, field := range message.Fields {
		if field.Oneof != nil {
		Loop:
			for {
				for _, nestedMessage := range message.Messages {
					if nestedMessage.GoIdent == field.GoIdent {
						field.GoIdent.GoName += "_"
						continue Loop
					}
				}
				for _, nestedEnum := range message.Enums {
					if nestedEnum.GoIdent == field.GoIdent {
						field.GoIdent.GoName += "_"
						continue Loop
					}
				}
				break Loop
			}
		}
	}

	return message
}

func (message *Message) Directives() []CommentDirective {
	return message.Comments.Directives
}

func (message *Message) resolveDependencies(gen *Plugin) error {
	for _, field := range message.Fields {
		if err := field.resolveDependencies(gen); err != nil {
			return err
		}
	}
	for _, msg := range message.Messages {
		if err := msg.resolveDependencies(gen); err != nil {
			return err
		}
	}
	for _, extension := range message.Extensions {
		if err := extension.resolveDependencies(gen); err != nil {
			return err
		}
	}
	return nil
}

// A Field describes a message field.
type Field struct {
	Desc protoreflect.FieldDescriptor

	// GoName is the base name of this field's Go field and methods.
	// For code generated by protoc-gen-go, this means a field named
	// '{{GoName}}' and a getter method named 'Get{{GoName}}'.
	GoName string // e.g., "FieldName"

	// GoIdent is the base name of a top-level declaration for this field.
	// For code generated by protoc-gen-go, this means a wrapper type named
	// '{{GoIdent}}' for members fields of a oneof, and a variable named
	// 'E_{{GoIdent}}' for extension fields.
	GoIdent GoIdent // e.g., "MessageName_FieldName"

	Parent   *Message // message in which this field is declared; nil if top-level extension
	Oneof    *Oneof   // containing oneof; nil if not part of a oneof
	Extendee *Message // extended message for extension fields; nil otherwise

	Enum    *Enum    // type for enum fields; nil otherwise
	Message *Message // type for message or group fields; nil otherwise

	Location Location   // location of this field
	Comments CommentSet // comments associated with this field
}

func newField(_ *Plugin, f *File, message *Message, desc protoreflect.FieldDescriptor) *Field {
	var loc Location
	switch {
	case desc.IsExtension() && message == nil:
		loc = f.location.appendPath(genid.FileDescriptorProto_Extension_field_number, desc.Index())
	case desc.IsExtension() && message != nil:
		loc = message.Location.appendPath(genid.DescriptorProto_Extension_field_number, desc.Index())
	default:
		loc = message.Location.appendPath(genid.DescriptorProto_Field_field_number, desc.Index())
	}
	camelCased := strs.GoCamelCase(string(desc.Name()))
	var parentPrefix string
	if message != nil {
		parentPrefix = message.GoIdent.GoName + "_"
	}
	field := &Field{
		Desc:   desc,
		GoName: camelCased,
		GoIdent: GoIdent{
			GoImportPath: f.GoImportPath,
			GoName:       parentPrefix + camelCased,
		},
		Parent:   message,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	parseDirectives(&field.Comments, field, field.Desc.ParentFile().Path())
	f.directives = append(f.directives, field.Directives()...)
	return field
}

func (field *Field) Directives() []CommentDirective {
	return field.Comments.Directives
}

func (field *Field) resolveDependencies(gen *Plugin) error {
	desc := field.Desc
	switch desc.Kind() {
	case protoreflect.EnumKind:
		name := field.Desc.Enum().FullName()
		enum, ok := gen.enumsByName[name]
		if !ok {
			return fmt.Errorf("field %v: no descriptor for enum %v", desc.FullName(), name)
		}
		field.Enum = enum
	case protoreflect.MessageKind, protoreflect.GroupKind:
		name := desc.Message().FullName()
		message, ok := gen.messagesByName[name]
		if !ok {
			return fmt.Errorf("field %v: no descriptor for type %v", desc.FullName(), name)
		}
		field.Message = message
	}
	if desc.IsExtension() {
		name := desc.ContainingMessage().FullName()
		message, ok := gen.messagesByName[name]
		if !ok {
			return fmt.Errorf("field %v: no descriptor for type %v", desc.FullName(), name)
		}
		field.Extendee = message
	}
	return nil
}

// A Oneof describes a message oneof.
type Oneof struct {
	Desc protoreflect.OneofDescriptor

	// GoName is the base name of this oneof's Go field and methods.
	// For code generated by protoc-gen-go, this means a field named
	// '{{GoName}}' and a getter method named 'Get{{GoName}}'.
	GoName string // e.g., "OneofName"

	// GoIdent is the base name of a top-level declaration for this oneof.
	GoIdent GoIdent // e.g., "MessageName_OneofName"

	Parent *Message // message in which this oneof is declared

	Fields []*Field // fields that are part of this oneof

	Location Location   // location of this oneof
	Comments CommentSet // comments associated with this oneof
}

func newOneof(_ *Plugin, f *File, message *Message, desc protoreflect.OneofDescriptor) *Oneof {
	loc := message.Location.appendPath(genid.DescriptorProto_OneofDecl_field_number, desc.Index())
	camelCased := strs.GoCamelCase(string(desc.Name()))
	parentPrefix := message.GoIdent.GoName + "_"
	oneOf := &Oneof{
		Desc:   desc,
		Parent: message,
		GoName: camelCased,
		GoIdent: GoIdent{
			GoImportPath: f.GoImportPath,
			GoName:       parentPrefix + camelCased,
		},
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	parseDirectives(&oneOf.Comments, oneOf, oneOf.Desc.ParentFile().Path())

	return oneOf
}

// Extension is an alias of Field for documentation.
type Extension = Field

// A Service describes a service.
type Service struct {
	Desc protoreflect.ServiceDescriptor

	File   *File
	GoName string

	Methods []*Method // service method declarations

	Location Location   // location of this service
	Comments CommentSet // comments associated with this service
}

func newService(gen *Plugin, f *File, desc protoreflect.ServiceDescriptor) *Service {
	loc := f.location.appendPath(genid.FileDescriptorProto_Service_field_number, desc.Index())
	service := &Service{
		Desc:     desc,
		File:     f,
		GoName:   strs.GoCamelCase(string(desc.Name())),
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	parseDirectives(&service.Comments, service, service.Desc.ParentFile().Path())
	for i, mds := 0, desc.Methods(); i < mds.Len(); i++ {
		service.Methods = append(service.Methods, newMethod(gen, f, service, mds.Get(i)))
		f.directives = append(f.directives, service.Directives()...)
	}
	return service
}

func (s *Service) Directives() []CommentDirective {
	return s.Comments.Directives
}

// A Method describes a method in a service.
type Method struct {
	Desc protoreflect.MethodDescriptor

	GoName string

	Parent *Service // service in which this method is declared

	Input  *Message
	Output *Message

	Location Location   // location of this method
	Comments CommentSet // comments associated with this method
}

func newMethod(_ *Plugin, f *File, service *Service, desc protoreflect.MethodDescriptor) *Method {
	loc := service.Location.appendPath(genid.ServiceDescriptorProto_Method_field_number, desc.Index())
	method := &Method{
		Desc:     desc,
		GoName:   strs.GoCamelCase(string(desc.Name())),
		Parent:   service,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	parseDirectives(&method.Comments, method, method.Desc.ParentFile().Path())

	return method
}

func (method *Method) Directives() []CommentDirective {
	return method.Comments.Directives
}

func (method *Method) resolveDependencies(gen *Plugin) error {
	desc := method.Desc

	inName := desc.Input().FullName()
	in, ok := gen.messagesByName[inName]
	if !ok {
		return fmt.Errorf("method %v: no descriptor for type %v", desc.FullName(), inName)
	}
	method.Input = in

	outName := desc.Output().FullName()
	out, ok := gen.messagesByName[outName]
	if !ok {
		return fmt.Errorf("method %v: no descriptor for type %v", desc.FullName(), outName)
	}
	method.Output = out

	return nil
}

// A GeneratedFile is a generated file.
type GeneratedFile struct {
	gen            *Plugin
	skip           bool
	fileName       string
	rawFileName    string
	insertionPoint string
	kind           templateKind
	source         string
	buf            bytes.Buffer
	packageNames   map[GoImportPath]GoPackageName
}

// NewGeneratedFile creates a new generated file with the given filename
// and import path.
func (gen *Plugin) NewGeneratedFile(t *template, tc *TemplateContext) *GeneratedFile {
	newGenFile := &GeneratedFile{
		gen:            gen,
		fileName:       t.fileName,
		rawFileName:    t.rawFileName,
		kind:           t.kind,
		insertionPoint: t.insertionPoint,
		source:         t.source,
		packageNames:   tc.packageNames,
	}

	if len(t.content) > 0 {
		_, err := newGenFile.Write([]byte(t.content))
		if err != nil {
			panic(err)
		}
	} else {
		newGenFile.Skip()
	}

	return newGenFile
}

// Write implements io.Writer.
func (g *GeneratedFile) Write(p []byte) (n int, err error) {
	return g.buf.Write(p)
}

// Skip removes the generated file from the plugin output.
func (g *GeneratedFile) Skip() {
	g.skip = true
}

// Unskip reverts a previous call to Skip, re-including the generated file in
// the plugin output.
func (g *GeneratedFile) Unskip() {
	g.skip = false
}

// Content returns the contents of the generated file.
func (g *GeneratedFile) Content() ([]byte, error) {
	if !strings.HasSuffix(g.fileName, ".go") {
		return g.buf.Bytes(), nil
	} else if len(g.insertionPoint) > 0 {
		value := g.buf.Bytes()
		formattedValue, err := format.Source(value)
		if err != nil {
			if g.gen.debug {
				log.Printf("Could not format source:\nFile: %s@%s\nContents:\n*****\n%s\n*****\n", g.fileName, g.insertionPoint, string(value))
			}
			return value, nil
		}
		return formattedValue, nil
	}

	// Reformat generated code.
	original := g.buf.Bytes()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", original, parser.ParseComments)
	if err != nil {
		// Print out the bad code with line numbers.
		// This should never happen in practice, but it can while changing generated code
		// so consider this a debugging aid.
		var src bytes.Buffer
		s := bufio.NewScanner(bytes.NewReader(original))
		for line := 1; s.Scan(); line++ {
			_, _ = fmt.Fprintf(&src, "%5d\t%s\n", line, s.Bytes())
		}
		return nil, fmt.Errorf("%v: unparsable Go source: %v\n%v", g.fileName, err, src.String())
	}

	// Collect a sorted list of all imports.
	var importPaths [][2]string
	rewriteImport := func(importPath string) string {
		if f := g.gen.opts.ImportRewriteFunc; f != nil {
			return string(f(GoImportPath(importPath)))
		}
		return importPath
	}
	for importPath := range g.packageNames {
		pkgName := string(g.packageNames[importPath])
		pkgPath := rewriteImport(string(importPath))
		if baseName(pkgPath) == pkgName {
			pkgName = ""
		}
		importPaths = append(importPaths, [2]string{pkgName, pkgPath})
	}
	sort.Slice(importPaths, func(i, j int) bool {
		return importPaths[i][1] < importPaths[j][1]
	})

	// Modify the AST to include a new import block.
	if len(importPaths) > 0 {
		// Insert block after package statement or
		// possible comment attached to the end of the package statement.
		pos := file.Package
		tokFile := fset.File(file.Package)
		pkgLine := tokFile.Line(file.Package)
		for _, c := range file.Comments {
			if tokFile.Line(c.Pos()) > pkgLine {
				break
			}
			pos = c.End()
		}

		// Construct the import block.
		impDecl := &ast.GenDecl{
			Tok:    token.IMPORT,
			TokPos: pos,
			Lparen: pos,
			Rparen: pos,
		}
		for _, importPath := range importPaths {
			var name *ast.Ident
			if len(importPath[0]) > 0 {
				name = &ast.Ident{
					Name:    importPath[0],
					NamePos: pos,
				}
			}
			impDecl.Specs = append(impDecl.Specs, &ast.ImportSpec{
				Name: name,
				Path: &ast.BasicLit{
					Kind:     token.STRING,
					Value:    strconv.Quote(importPath[1]),
					ValuePos: pos,
				},
				EndPos: pos,
			})
		}
		file.Decls = append([]ast.Decl{impDecl}, file.Decls...)
	}

	var out bytes.Buffer
	if err = (&printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}).Fprint(&out, fset, file); err != nil {
		return nil, fmt.Errorf("%v: can not reformat Go source: %v", g.fileName, err)
	}
	return out.Bytes(), nil
}

// A GoIdent is a Go identifier, consisting of a name and import path.
// The name is a single identifier and may not be a dot-qualified selector.
type GoIdent struct {
	GoName       string
	GoImportPath GoImportPath
}

func (id GoIdent) String() string { return fmt.Sprintf("%q.%v", id.GoImportPath, id.GoName) }

// newGoIdent returns the Go identifier for a descriptor.
func newGoIdent(f *File, d protoreflect.Descriptor) GoIdent {
	name := strings.TrimPrefix(string(d.FullName()), string(f.Desc.Package())+".")
	return GoIdent{
		GoName:       strs.GoCamelCase(name),
		GoImportPath: f.GoImportPath,
	}
}

// A GoImportPath is the import path of a Go package.
// For example: "google.golang.org/protobuf/compiler/protogen"
type GoImportPath string

func (p GoImportPath) String() string { return strconv.Quote(string(p)) }

// Ident returns a GoIdent with s as the GoName and p as the GoImportPath.
func (p GoImportPath) Ident(s string) GoIdent {
	return GoIdent{GoName: s, GoImportPath: p}
}

// A GoPackageName is the name of a Go package. e.g., "protobuf".
type GoPackageName string

// cleanPackageName converts a string to a valid Go package name.
func cleanPackageName(name string) GoPackageName {
	return GoPackageName(strs.GoSanitized(name))
}

// A Location is a location in a .proto source file.
//
// See the google.protobuf.SourceCodeInfo documentation in descriptor.proto
// for details.
type Location struct {
	SourceFile string
	Path       protoreflect.SourcePath
}

// appendPath add elements to a Location's path, returning a new Location.
func (loc Location) appendPath(num protoreflect.FieldNumber, idx int) Location {
	loc.Path = append(protoreflect.SourcePath(nil), loc.Path...) // make copy
	loc.Path = append(loc.Path, int32(num), int32(idx))
	return loc
}

// CommentSet is a set of leading and trailing comments associated
// with a .proto descriptor declaration.
type CommentSet struct {
	LeadingDetached []Comments
	Leading         Comments
	Trailing        Comments

	Directives []CommentDirective
}

func makeCommentSet(loc protoreflect.SourceLocation) CommentSet {
	var leadingDetached []Comments
	for _, s := range loc.LeadingDetachedComments {
		leadingDetached = append(leadingDetached, Comments(s))
	}
	commentSet := CommentSet{
		LeadingDetached: leadingDetached,
		Leading:         Comments(loc.LeadingComments),
		Trailing:        Comments(loc.TrailingComments),
		Directives:      make([]CommentDirective, 0),
	}
	return commentSet
}

func (c CommentSet) hasAnnotation(str string) bool {
	if !strings.HasPrefix(str, "@") {
		str = "@" + str
	}
	return c.commentPrefix(str)
}

func (c CommentSet) hasDirective(str string) bool {
	str = strings.ToLower(str)
	for !strings.HasPrefix(strings.Trim(str, " \t\r\n"), "@@") {
		str = "@" + str
	}
	return c.commentPrefix(str)
}

func (c CommentSet) commentPrefix(str string) bool {
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

// Comments is a comments string as provided by protoc.
type Comments string

// String formats the comments by inserting // to the start of each line,
// ensuring that there is a trailing newline.
// An empty comment is formatted as an empty string.
func (c Comments) String() string {
	if c == "" {
		return ""
	}
	var b []byte
	for _, line := range strings.Split(strings.TrimSuffix(string(c), "\n"), "\n") {
		b = append(b, "//"...)
		b = append(b, line...)
		b = append(b, "\n"...)
	}
	return string(b)
}

var directiveRe = regexp.MustCompile(`(?s)@@(?P<directive>[^(]*)(?:\((?P<params>[^)]+(?:,\s)?)\)\s*\x60{0,3}\s*(?P<value>[^\x60@]*)?\s*\x60{0,3})?`)

type CommentDirective struct {
	Directive string
	Params    []string
	Value     string
	Source    string
	Type      interface{}
}

func parseExpression(re *regexp.Regexp, str string) []map[string]string {
	match := re.FindAllStringSubmatch(str, -1)

	paramsMap := make([]map[string]string, len(match))
	for i := range paramsMap {
		paramsMap[i] = make(map[string]string)
	}
	for j := range match {
		for i, name := range re.SubexpNames() {
			if i == 0 {
				continue
			}
			if i > 0 && i <= len(match[j]) {
				paramsMap[j][name] = match[j][i]
			}
		}
	}
	return paramsMap
}

func parseDirectives(c *CommentSet, parent interface{}, source string) {
	processComment := func(comments ...Comments) []CommentDirective {
		d := make([]CommentDirective, 0)
		for _, comment := range comments {
			commentStr := strings.Trim(string(comment), " \t\r\n")
			if !strings.HasPrefix(commentStr, "@@") {
				continue
			}
			mapping := parseExpression(directiveRe, commentStr)
			for match := range mapping {
				d = append(d, CommentDirective{
					Directive: mapping[match]["directive"],
					Params:    strings.Split(mapping[match]["params"], ", "),
					Value:     mapping[match]["value"],
					Source:    source,
					Type:      parent,
				})
			}
		}
		return d
	}
	c.Directives = append(c.Directives, processComment(c.Leading)...)
	c.Directives = append(c.Directives, processComment(c.Trailing)...)
	c.Directives = append(c.Directives, processComment(c.LeadingDetached...)...)
}
