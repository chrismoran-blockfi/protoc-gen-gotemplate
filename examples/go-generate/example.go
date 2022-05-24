package example

//go:generate protoc --gotemplate_out=template_dir=templates,mode=service:./gen --go_out=. ./gen/example.proto
