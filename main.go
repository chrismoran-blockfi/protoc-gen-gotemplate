package main

func main() {
}

/*
var (
	registry *helpers.Registry
)

const (
	boolTrue  = "true"
	boolFalse = "false"
)

type Generator struct {
	Request  *plugingo.CodeGeneratorRequest  // The input.
	Response *plugingo.CodeGeneratorResponse // The output.
}

func NewGenerator() *Generator {
	g := new(Generator)
	g.Request = new(plugingo.CodeGeneratorRequest)
	g.Response = new(plugingo.CodeGeneratorResponse)
	return g
}

// Error reports a problem, including an error, and exits the program.
func Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("protoc-gen-gotemplate: error:", s)
	os.Exit(1)
}

// Fail reports a problem and exits the program.
func Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("protoc-gen-gotemplate: error:", s)
	os.Exit(1)
}

func main() {
	g := NewGenerator()

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		Error(err, "reading input")
	}

	if err = proto.Unmarshal(data, g.Request); err != nil {
		Error(err, "parsing input proto")
	}

	if len(g.Request.FileToGenerate) == 0 {
		Fail("no files to generate")
	}

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
					log.Printf("Err: invalid value for all: %q", parts[1])
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
	ipMap := make(map[string]bool)
	concatOrAppend := func(file *plugingo.CodeGeneratorResponse_File) {
		key := fmt.Sprintf("%s:%s", file.GetName(), file.GetInsertionPoint())
		baseFile := fmt.Sprintf("%s:", file.GetName())

		if val, ok := tmplMap[key]; ok {
			*val.Content += file.GetContent()
		} else {
			if key == baseFile {
				tmplMap[key] = file
				ipMap[key] = true
			}
			if exists, isOk := ipMap[baseFile]; !isOk || !exists {
				if debug {
					_, _ = fmt.Fprintf(os.Stderr, "%s does not exist, skipping %s\n", baseFile, key)
				}
			} else {
				tmplMap[key] = file
				ipMap[baseFile] = true
				g.Response.File = append(g.Response.File, file)
			}
		}
	}

	if singlePackageMode {
		registry = gengotemplate.NewRegistry()
		gengotemplate.SetRegistry(registry)
		if err = registry.Load(g.Request); err != nil {
			Error(err, "registry: failed to load the request")
		}
	}

	baseIndex := 0
	// Generate the encoders
	rfs := gengotemplate.RequestFileSorter{
		Request: g.Request,
	}
	sort.Sort(rfs)
	for _, file := range rfs.Request.GetProtoFile() {
		templateIndex := index
		if index == -1 {
			templateIndex = baseIndex
		}
		baseIndex = baseIndex + 1
		if all {
			if singlePackageMode {
				if _, err = registry.LookupFile(file.GetName()); err != nil {
					Error(err, "registry: failed to lookup file %q", file.GetName())
				}
			}
			encoder := gengotemplate.NewGenericTemplateBasedEncoder(templateDir, file, debug, destinationDir, templateIndex)
			for _, tmpl := range encoder.Files() {
				concatOrAppend(tmpl)
			}

			continue
		}

		if fileMode {
			if s := file.GetService(); s != nil && len(s) > 0 {
				encoder := gengotemplate.NewGenericTemplateBasedEncoder(templateDir, file, debug, destinationDir, templateIndex)
				for _, tmpl := range encoder.Files() {
					concatOrAppend(tmpl)
				}
			}

			continue
		}

		for _, service := range file.GetService() {
			encoder := gengotemplate.NewGenericServiceTemplateBasedEncoder(templateDir, service, file, debug, destinationDir, templateIndex)
			for _, tmpl := range encoder.Files() {
				concatOrAppend(tmpl)
			}
		}
	}
	// Generate the protobufs
	g.Response.SupportedFeatures = proto.Uint64(uint64(plugingo.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL))

	data, err = proto.Marshal(g.Response)
	if err != nil {
		Error(err, "failed to marshal output proto")
	}

	_, err = os.Stdout.Write(data)
	if err != nil {
		Error(err, "failed to write output proto")
	}
}
*/
