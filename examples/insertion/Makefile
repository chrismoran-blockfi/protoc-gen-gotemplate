.PHONY: build
build:
	protoc --gotemplate_out=debug=true,template_dir=.,mode=all:. *.proto

.PHONY: re
re: build
