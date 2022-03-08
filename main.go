package main

import (
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"go-micro.dev/v4/cmd/protoc-gen-micro/generator"
	"google.golang.org/protobuf/proto"
	plugingo "google.golang.org/protobuf/types/pluginpb"

	pgghelpers "github.com/chrismoran-blockfi/protoc-gen-gotemplate/helpers"
)

var (
	registry *pgghelpers.Registry
)

const (
	boolTrue  = "true"
	boolFalse = "false"
)

func main() {
	g := generator.New()

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		g.Error(err, "reading input")
	}

	if err = proto.Unmarshal(data, g.Request); err != nil {
		g.Error(err, "parsing input proto")
	}

	if len(g.Request.FileToGenerate) == 0 {
		g.Fail("no files to generate")
	}

	g.CommandLineParameters(g.Request.GetParameter())

	// Parse parameters
	var (
		templateDir       = "./templates"
		destinationDir    = "."
		index             = -1
		debug             = false
		all               = false
		singlePackageMode = false
		fileMode          = false
	)
	if parameter := g.Request.GetParameter(); parameter != "" {
		for _, param := range strings.Split(parameter, ",") {
			parts := strings.Split(param, "=")
			if len(parts) != 2 {
				log.Printf("Err: invalid parameter: %q", param)
				continue
			}
			switch parts[0] {
			case "index":
				index, err = strconv.Atoi(parts[1])
				if err != nil {
					log.Printf("Could not convert %s to an integer", parts[1])
				}
			case "template_dir":
				templateDir = parts[1]
			case "destination_dir":
				destinationDir = parts[1]
			case "single-package-mode":
				switch strings.ToLower(parts[1]) {
				case boolTrue, "t":
					singlePackageMode = true
				case boolFalse, "f":
				default:
					log.Printf("Err: invalid value for single-package-mode: %q", parts[1])
				}
			case "debug":
				switch strings.ToLower(parts[1]) {
				case boolTrue, "t":
					debug = true
				case boolFalse, "f":
				default:
					log.Printf("Err: invalid value for debug: %q", parts[1])
				}
			case "all":
				switch strings.ToLower(parts[1]) {
				case boolTrue, "t":
					all = true
				case boolFalse, "f":
				default:
					log.Printf("Err: invalid value for debug: %q", parts[1])
				}
			case "file-mode":
				switch strings.ToLower(parts[1]) {
				case boolTrue, "t":
					fileMode = true
				case boolFalse, "f":
				default:
					log.Printf("Err: invalid value for file-mode: %q", parts[1])
				}
			default:
				log.Printf("Err: unknown parameter: %q", param)
			}
		}
	}

	tmplMap := make(map[string]*plugingo.CodeGeneratorResponse_File)
	concatOrAppend := func(file *plugingo.CodeGeneratorResponse_File) {
		if val, ok := tmplMap[file.GetName()]; ok {
			*val.Content += file.GetContent()
		} else {
			tmplMap[file.GetName()] = file
			g.Response.File = append(g.Response.File, file)
		}
	}

	if singlePackageMode {
		registry = pgghelpers.NewRegistry()
		pgghelpers.SetRegistry(registry)
		if err = registry.Load(g.Request); err != nil {
			g.Error(err, "registry: failed to load the request")
		}
	}

	// Generate the encoders
	for fileIndex, file := range g.Request.GetProtoFile() {
		templateIndex := index
		if index == -1 {
			templateIndex = fileIndex
		}
		if all {
			if singlePackageMode {
				if _, err = registry.LookupFile(file.GetName()); err != nil {
					g.Error(err, "registry: failed to lookup file %q", file.GetName())
				}
			}
			encoder := pgghelpers.NewGenericTemplateBasedEncoder(templateDir, file, debug, destinationDir, templateIndex)
			for _, tmpl := range encoder.Files() {
				concatOrAppend(tmpl)
			}

			continue
		}

		if fileMode {
			if s := file.GetService(); s != nil && len(s) > 0 {
				encoder := pgghelpers.NewGenericTemplateBasedEncoder(templateDir, file, debug, destinationDir, templateIndex)
				for _, tmpl := range encoder.Files() {
					concatOrAppend(tmpl)
				}
			}

			continue
		}

		for _, service := range file.GetService() {
			encoder := pgghelpers.NewGenericServiceTemplateBasedEncoder(templateDir, service, file, debug, destinationDir, templateIndex)
			for _, tmpl := range encoder.Files() {
				concatOrAppend(tmpl)
			}
		}
	}

	// Generate the protobufs
	g.GenerateAllFiles()

	data, err = proto.Marshal(g.Response)
	if err != nil {
		g.Error(err, "failed to marshal output proto")
	}

	_, err = os.Stdout.Write(data)
	if err != nil {
		g.Error(err, "failed to write output proto")
	}
}
