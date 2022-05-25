package compiler

import (
	"bytes"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/pluginpb"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	gotemplate "text/template"
)

const goPackageDocURL = "https://developers.google.com/protocol-buffers/docs/reference/go-generated#package"

var SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

const (
	boolTrue  = "true"
	boolFalse = "false"
)

type ResponseSorter []*pluginpb.CodeGeneratorResponse_File

func (a ResponseSorter) Len() int {
	return len(a)
}

func (a ResponseSorter) Less(i, j int) bool {
	nameCmp := strings.Compare(a[i].GetName(), a[j].GetName())
	return nameCmp < 0 || nameCmp == 0 && len(a[i].GetInsertionPoint()) == 0
}

func (a ResponseSorter) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

type templateSorter []*template

func (a templateSorter) Len() int {
	return len(a)
}

func (a templateSorter) Less(i, j int) bool {
	fileNameCmp := strings.Compare(a[i].fileName, a[j].fileName)
	kindCmp := int(a[i].kind) - int(a[j].kind)
	insertionPointCmp := strings.Compare(a[i].insertionPoint, a[j].insertionPoint)
	sourceCmp := 0
	if a[i].source == nil {
		sourceCmp = -1
	} else if a[j].source == nil {
		sourceCmp = 1
	} else {
		sourceCmp = strings.Compare(a[i].source.ParentFile().Path(), a[j].source.ParentFile().Path())
	}
	return fileNameCmp < 0 ||
		fileNameCmp == 0 && kindCmp < 0 ||
		fileNameCmp == 0 && kindCmp == 0 && insertionPointCmp < 0 ||
		fileNameCmp == 0 && kindCmp == 0 && insertionPointCmp == 0 && sourceCmp < 0
}

func (a templateSorter) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

type Directable interface {
	Directives() []CommentDirective
}

type Mode int

const (
	ServiceMode Mode = iota
	FileMode
	AllMode
)

type templateKind int

const (
	fileTemplateKind templateKind = iota
	directiveTemplateKind
)

type genFileSorter []*GeneratedFile

func (a genFileSorter) Len() int {
	return len(a)
}

func (a genFileSorter) Less(i, j int) bool {
	fileNameCmp := strings.Compare(a[i].fileName, a[j].fileName)
	kindCmp := int(a[i].kind) - int(a[j].kind)
	insertionPointCmp := strings.Compare(a[i].insertionPoint, a[j].insertionPoint)
	sourceCmp := 0
	if a[i].source == nil {
		sourceCmp = -1
	} else if a[j].source == nil {
		sourceCmp = 1
	} else {
		sourceCmp = strings.Compare(a[i].source.ParentFile().Path(), a[j].source.ParentFile().Path())
	}
	return fileNameCmp < 0 ||
		fileNameCmp == 0 && kindCmp < 0 ||
		fileNameCmp == 0 && kindCmp == 0 && insertionPointCmp < 0 ||
		fileNameCmp == 0 && kindCmp == 0 && insertionPointCmp == 0 && sourceCmp < 0
}

func (a genFileSorter) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

type templateOption func(*template)

func withRawFilename(s string) templateOption {
	return func(t *template) {
		t.rawFileName = s
	}
}
func withContent(s string) templateOption {
	return func(t *template) {
		t.content = s
	}
}
func withTemplateDir(s string) templateOption {
	return func(t *template) {
		t.templateDir = s
	}
}
func withInsertionPoint(s string) templateOption {
	return func(t *template) {
		t.insertionPoint = s
	}
}
func withKind(k templateKind) templateOption {
	return func(t *template) {
		t.kind = k
	}
}

func withSource(s protoreflect.Descriptor) templateOption {
	return func(t *template) {
		t.source = s
	}
}

type TemplateContext struct {
	Mode             Mode   `json:"mode"`
	Index            int    `json:"index"`
	RawFilename      string `json:"raw-filename"`
	Filename         string `json:"filename"`
	file             *File
	service          *Service
	impMu            sync.Mutex
	packageNames     map[GoImportPath]GoPackageName
	usedPackages     map[GoImportPath]bool  // Packages used in current file.
	usedPackageNames map[GoPackageName]bool // Package names used in the current file.
	addedImports     map[GoImportPath]bool  // Additional imports to emit.
}

type template struct {
	templateDir    string
	rawFileName    string
	fileName       string
	content        string
	insertionPoint string
	source         protoreflect.Descriptor
	kind           templateKind
}

func newTemplate(opts ...templateOption) *template {
	t := &template{}
	for _, o := range opts {
		o(t)
	}
	if len(t.rawFileName) > 0 && strings.Contains(t.rawFileName, "@") && strings.HasSuffix(t.rawFileName, ".tmpl") {
		t.insertionPoint = t.rawFileName[strings.Index(t.rawFileName, "@")+1 : strings.Index(t.rawFileName, ".tmpl")]
		t.fileName = t.outputFileName()
	}
	if len(t.fileName) == 0 {
		t.fileName = t.rawFileName
	}

	return t
}

func (t *template) outputFileName() string {
	fileName := t.rawFileName
	if len(t.insertionPoint) > 0 && strings.Contains(fileName, "@") {
		fileName = fileName[:strings.Index(fileName, "@")] + ".tmpl"
	}
	if strings.HasSuffix(fileName, ".tmpl") {
		fileName = fileName[:len(fileName)-len(".tmpl")]
	}

	return fileName
}

func (t *template) createTemplateFile() (*gotemplate.Template, error) {
	templateFilename := t.rawFileName
	fullPath := filepath.Join(t.templateDir, templateFilename)
	templateName := filepath.Base(fullPath)
	templateFile := gotemplate.New(templateName).Funcs(ProtoHelpersFuncMap)

	var err error
	if t.content == "" {
		templateFile, err = templateFile.ParseFiles(fullPath)
	} else {
		templateFile, err = templateFile.Parse(t.content)
	}
	if err != nil {
		return nil, err
	}

	return templateFile, nil
}

func (t *template) createContext(mode Mode, index int, directable Directable) (*TemplateContext, error) {
	nopt := &TemplateContext{
		Index:            index,
		Mode:             mode,
		RawFilename:      t.rawFileName,
		usedPackages:     make(map[GoImportPath]bool),
		packageNames:     make(map[GoImportPath]GoPackageName),
		usedPackageNames: make(map[GoPackageName]bool),
		addedImports:     make(map[GoImportPath]bool),
	}

	switch mode {
	case AllMode:
		nopt.file = directable.(*File)
		nopt.service = nil
	case ServiceMode:
		nopt.service = directable.(*Service)
		nopt.file = directable.(*Service).File
	case FileMode:
		nopt.file = directable.(*File)
		nopt.service = nil
	}
	setContext(nopt)

	buffer := new(bytes.Buffer)

	unescaped, err := url.QueryUnescape(t.rawFileName)
	if err != nil {
		log.Printf("failed to unescape filepath %q: %v", t.rawFileName, err)
	} else {
		t.rawFileName = unescaped
	}

	templateFile, err := gotemplate.New("").Funcs(ProtoHelpersFuncMap).Parse(t.rawFileName)
	if err != nil {
		return nil, err
	}
	if err = templateFile.Execute(buffer, nopt); err != nil {
		return nil, err
	}
	nopt.Filename = buffer.String()
	t.fileName = nopt.Filename
	t.fileName = t.outputFileName()
	return nopt, nil
}

func (t *template) executeTemplate(mode Mode, index int, dir Directable) (tc *TemplateContext, err error) {

	var templateFile *gotemplate.Template

	if templateFile, err = t.createTemplateFile(); err != nil {
		return tc, err
	}
	if tc, err = t.createContext(mode, index, dir); err != nil {
		return tc, err
	}

	buffer := new(bytes.Buffer)

	if err = templateFile.Execute(buffer, tc); err != nil {
		return tc, err
	}

	t.content = buffer.String()

	return tc, err
}
