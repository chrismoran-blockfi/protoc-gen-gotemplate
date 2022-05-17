package helpers

import (
	"bytes"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	tmpl "text/template"
	"time"

	descriptor "google.golang.org/protobuf/types/descriptorpb"
	plugingo "google.golang.org/protobuf/types/pluginpb"
)

type ResponseSorter []*plugingo.CodeGeneratorResponse_File

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

type RequestFileSorter struct {
	Request *plugingo.CodeGeneratorRequest
}

func (r RequestFileSorter) Len() int {
	return len(r.Request.ProtoFile)
}

func (r RequestFileSorter) Less(i, j int) bool {
	return strings.Compare(r.Request.ProtoFile[i].GetName(), r.Request.ProtoFile[j].GetName()) == -1
}

func (r RequestFileSorter) Swap(i, j int) {
	r.Request.ProtoFile[i], r.Request.ProtoFile[j] = r.Request.ProtoFile[j], r.Request.ProtoFile[i]
}

type GenericTemplateBasedEncoder struct {
	templateDir    string
	service        *descriptor.ServiceDescriptorProto
	file           *descriptor.FileDescriptorProto
	enum           []*descriptor.EnumDescriptorProto
	debug          bool
	destinationDir string
	index          int
	pathMap        map[interface{}]*descriptor.SourceCodeInfo_Location
	directivesMap  map[interface{}][]CommentDirective
}

type Ast struct {
	BuildDate      time.Time                          `json:"build-date"`
	BuildHostname  string                             `json:"build-hostname"`
	BuildUser      string                             `json:"build-user"`
	GoPWD          string                             `json:"go-pwd,omitempty"`
	PWD            string                             `json:"pwd"`
	Debug          bool                               `json:"debug"`
	DestinationDir string                             `json:"destination-dir"`
	File           *descriptor.FileDescriptorProto    `json:"file"`
	RawFilename    string                             `json:"raw-filename"`
	Filename       string                             `json:"filename"`
	TemplateDir    string                             `json:"template-dir"`
	Service        *descriptor.ServiceDescriptorProto `json:"service"`
	Enum           []*descriptor.EnumDescriptorProto  `json:"enum"`
	Index          int                                `json:"index"`
}

func NewGenericServiceTemplateBasedEncoder(templateDir string, service *descriptor.ServiceDescriptorProto, file *descriptor.FileDescriptorProto, debug bool, destinationDir string, index int) (e *GenericTemplateBasedEncoder) {
	e = &GenericTemplateBasedEncoder{
		service:        service,
		file:           file,
		templateDir:    templateDir,
		debug:          debug,
		destinationDir: destinationDir,
		enum:           file.GetEnumType(),
		index:          index,
		directivesMap:  make(map[interface{}][]CommentDirective),
	}
	if debug {
		log.Printf("new encoder: file=%q service=%q template-dir=%q", file.GetName(), service.GetName(), templateDir)
	}
	LoadComments(file)
	parseDirectives(&e.directivesMap)
	return
}

func NewGenericTemplateBasedEncoder(templateDir string, file *descriptor.FileDescriptorProto, debug bool, destinationDir string, index int) (e *GenericTemplateBasedEncoder) {
	e = &GenericTemplateBasedEncoder{
		service:        nil,
		file:           file,
		templateDir:    templateDir,
		enum:           file.GetEnumType(),
		debug:          debug,
		destinationDir: destinationDir,
		index:          index,
		directivesMap:  make(map[interface{}][]CommentDirective),
	}
	if debug {
		log.Printf("new encoder: file=%q template-dir=%q", file.GetName(), templateDir)
	}
	LoadComments(file)
	parseDirectives(&e.directivesMap)

	return
}

type template struct {
	fileName       string
	content        string
	insertionPoint string
}

func (e *GenericTemplateBasedEncoder) templates() ([]template, error) {
	templates := make([]template, 0)

	err := filepath.Walk(e.templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".tmpl" {
			return nil
		}
		rel, err := filepath.Rel(e.templateDir, path)
		if err != nil {
			return err
		}
		if e.debug {
			log.Printf("new template: %q", rel)
		}

		templates = append(templates, template{
			fileName: rel,
		})
		return nil
	})
	for _, dirs := range e.directivesMap {
		for _, dir := range dirs {
			if dir.Directive == "protoc_insert" {
				params := strings.Split(dir.Params, ", ")
				name := params[0]
				insert := params[1]
				content := dir.Value
				templates = append(templates, template{
					fileName:       name,
					content:        content,
					insertionPoint: insert,
				})
			}
		}
	}
	return templates, err
}

func (e *GenericTemplateBasedEncoder) genAst(templateFilename string) (*Ast, error) {
	// prepare the ast passed to the template engine
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	goPwd := ""
	if os.Getenv("GOPATH") != "" {
		goPwd, err = filepath.Rel(os.Getenv("GOPATH")+"/src", pwd)
		if err != nil {
			return nil, err
		}
		if strings.Contains(goPwd, "../") {
			goPwd = ""
		}
	}
	ast := Ast{
		BuildDate:      time.Now(),
		BuildHostname:  hostname,
		BuildUser:      os.Getenv("USER"),
		PWD:            pwd,
		GoPWD:          goPwd,
		File:           e.file,
		TemplateDir:    e.templateDir,
		DestinationDir: e.destinationDir,
		RawFilename:    templateFilename,
		Filename:       "",
		Service:        e.service,
		Enum:           e.enum,
		Index:          e.index,
	}
	buffer := new(bytes.Buffer)

	unescaped, err := url.QueryUnescape(templateFilename)
	if err != nil {
		log.Printf("failed to unescape filepath %q: %v", templateFilename, err)
	} else {
		templateFilename = unescaped
	}

	templateFile, err := tmpl.New("").Funcs(ProtoHelpersFuncMap).Parse(templateFilename)
	if err != nil {
		return nil, err
	}
	if err := templateFile.Execute(buffer, ast); err != nil {
		return nil, err
	}
	ast.Filename = buffer.String()
	return &ast, nil
}

func (e *GenericTemplateBasedEncoder) buildContent(tmplt template) (string, string, error) {
	// initialize template engine
	if tmplt.content == "" {
	}
	templateFilename := tmplt.fileName
	fullPath := filepath.Join(e.templateDir, templateFilename)
	templateName := filepath.Base(fullPath)

	templateFile := tmpl.New(templateName).Funcs(ProtoHelpersFuncMap)
	var terr error
	if tmplt.content == "" {
		templateFile, terr = templateFile.ParseFiles(fullPath)
	} else {
		templateFile, terr = templateFile.Parse(tmplt.content)
	}
	if terr != nil {
		return "", "", terr
	}

	ast, err := e.genAst(templateFilename)
	if err != nil {
		return "", "", err
	}

	// generate the content
	buffer := new(bytes.Buffer)
	if err := templateFile.Execute(buffer, ast); err != nil {
		return "", "", err
	}

	return buffer.String(), ast.Filename, nil
}

func (e *GenericTemplateBasedEncoder) Files() []*plugingo.CodeGeneratorResponse_File {
	templates, err := e.templates()
	if err != nil {
		log.Fatalf("cannot get templates from %q: %v", e.templateDir, err)
	}

	length := len(templates)
	files := make([]*plugingo.CodeGeneratorResponse_File, 0, length)
	errChan := make(chan error, length)
	resultChan := make(chan *plugingo.CodeGeneratorResponse_File, length)

	for _, templ := range templates {
		go func(tmpl template) {
			var translatedFilename, content, insertionPoint, filename string

			if strings.Contains(tmpl.fileName, "@") {
				insertionPoint = tmpl.fileName[strings.Index(tmpl.fileName, "@")+1 : strings.Index(tmpl.fileName, ".tmpl")]
			}
			if tmpl.insertionPoint != "" {
				insertionPoint = tmpl.insertionPoint
			}

			content, translatedFilename, err = e.buildContent(tmpl)
			if err != nil {
				errChan <- err
				return
			}
			if len(insertionPoint) > 0 && strings.Contains(tmpl.fileName, "@") {
				filename = tmpl.fileName[:strings.Index(tmpl.fileName, "@")]
			} else {
				filename = translatedFilename[:len(translatedFilename)-len(".tmpl")]
			}

			if len(insertionPoint) > 0 {
				resultChan <- &plugingo.CodeGeneratorResponse_File{
					Content:        &content,
					Name:           &filename,
					InsertionPoint: &insertionPoint,
				}
			} else {
				resultChan <- &plugingo.CodeGeneratorResponse_File{
					Content: &content,
					Name:    &filename,
				}
			}
		}(templ)
	}
	for i := 0; i < length; i++ {
		select {
		case f := <-resultChan:
			files = append(files, f)
		case err = <-errChan:
			panic(err)
		}
	}
	sort.Sort(ResponseSorter(files))
	return files
}
