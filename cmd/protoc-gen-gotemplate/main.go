package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/chrismoran-blockfi/protoc-gen-gotemplate/cmd/protoc-gen-gotemplate/compiler"
	intio "github.com/chrismoran-blockfi/protoc-gen-gotemplate/internal/io"
	"github.com/chrismoran-blockfi/protoc-gen-gotemplate/internal/version"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		_, _ = fmt.Fprintf(os.Stdout, "%v %v\n", filepath.Base(os.Args[0]), version.String())
		os.Exit(0)
	}

	if len(os.Args) == 3 && os.Args[1] == "--debug" {
		data, _ := ioutil.ReadFile(os.Args[2])
		req := &pluginpb.CodeGeneratorRequest{}
		_ = json.Unmarshal(data, req)
		data, _ = proto.Marshal(req)
		if err := intio.ReplaceStdin(data); err != nil {
			panic(err)
		}
		_ = os.Setenv("PROTOC_GEN_GOTEMPLATE_DEBUG", "true")
		os.Args = []string{os.Args[0]}
	} else if doDebug, ok := os.LookupEnv("PROTOC_GEN_GOTEMPLATE_DEBUGFILE"); ok && len(doDebug) == 0 {
		var data []byte
		var err error
		if data, err = ioutil.ReadAll(os.Stdin); err != nil {
			panic(err)
		}
		req := &pluginpb.CodeGeneratorRequest{}
		_ = proto.Unmarshal(data, req)
		log.Println("protoc-gen-gotemplate unmarshaling...")
		jdata, _ := json.MarshalIndent(req, "", "  ")
		log.Println("protoc-gen-gotemplate saving request...")
		_ = ioutil.WriteFile("req.json", jdata, 0644)
		log.Println("protoc-gen-gotemplate finished saving request... exiting")
		os.Exit(1)
	} else if len(doDebug) > 0 {
		log.Println("protoc-gen-gotemplate peeking...")
		data, err := intio.PeekStdin()
		log.Println("protoc-gen-gotemplate stdin peeked...")
		if err != nil {
			panic(err)
		}
		req := &pluginpb.CodeGeneratorRequest{}
		_ = proto.Unmarshal(data, req)
		log.Println("protoc-gen-gotemplate unmarshaling...")
		jdata, _ := json.MarshalIndent(req, "", "  ")
		log.Println("protoc-gen-gotemplate saving request...")
		_ = ioutil.WriteFile(doDebug, jdata, 0644)
		log.Println("protoc-gen-gotemplate finished saving request...")
	}

	var (
		flags flag.FlagSet
		opts  = &compiler.Options{
			ParamFunc: flags.Set,
		}
	)

	opts.Run(func(gen *compiler.Plugin) error {
		compiler.ProcessTemplates(gen)
		gen.SupportedFeatures = compiler.SupportedFeatures
		return nil
	})
}
