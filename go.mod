module github.com/chrismoran-blockfi/protoc-gen-gotemplate

go 1.17

replace (
	github.com/chrismoran-blockfi/protoc-gen-gotemplate/examples v0.0.0 => ./examples
	github.com/chrismoran-blockfi/protoc-gen-gotemplate/helpers v0.0.0 => ./helpers
)

require (
	github.com/chrismoran-blockfi/protoc-gen-gotemplate/helpers v0.0.0
	github.com/gobuffalo/packr/v2 v2.8.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	go-micro.dev/v4 v4.6.0
	google.golang.org/protobuf v1.27.1
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.1 // indirect
	github.com/gobuffalo/logger v1.0.3 // indirect
	github.com/gobuffalo/packd v1.0.0 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/karrick/godirwalk v1.15.3 // indirect
	github.com/markbates/errx v1.1.0 // indirect
	github.com/markbates/oncer v1.0.0 // indirect
	github.com/markbates/safe v1.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/sirupsen/logrus v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a // indirect
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007 // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	google.golang.org/genproto v0.0.0-20220217155828-d576998c0009 // indirect
)
