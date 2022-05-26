package compiler

import (
	"errors"
	"os"
	"path"
	"strconv"
	"strings"
)

func (tc *TemplateContext) goPackageName(importPath GoImportPath) GoPackageName {
	tc.impMu.Lock()
	defer tc.impMu.Unlock()
	if name, ok := tc.packageNames[importPath]; ok {
		return name
	}
	name := cleanPackageName(baseName(string(importPath)))
	for i, orig := 1, name; tc.usedPackageNames[name] || isGoPredeclaredIdentifier[string(name)]; i++ {
		name = orig + GoPackageName(strconv.Itoa(i))
	}
	tc.packageNames[importPath] = name
	tc.usedPackageNames[name] = true
	return name
}

func (tc *TemplateContext) AddImport(i string) GoPackageName {
	importPath := GoImportPath(i)
	return tc.goPackageName(importPath)
}

func (tc *TemplateContext) NoClobber() bool {
	return !tc.Clobber()
}

func (tc *TemplateContext) Clobber() bool {
	output := path.Join(path.Dir(tc.destinationDir), tc.Filename)
	if strings.HasSuffix(output, ".tmpl") {
		output = output[:len(output)-len(".tmpl")]
	}

	if _, err := os.Stat(output); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func (tc *TemplateContext) GoMethodType(m *Method) string {
	return goMethodType(tc, m)
}

func (tc *TemplateContext) GoMethodDefinition(m *Method, parameterNames ...string) string {
	return goMethodDefinition(tc, m, parameterNames...)
}

func (tc *TemplateContext) Context() interface{} {
	if tc.Mode == ServiceMode {
		return tc.service
	}
	return tc.file
}

func (tc *TemplateContext) Package() string {
	return goPackage(tc.Context())
}

func (tc *TemplateContext) Name() string {
	return goName(tc.Context())
}

func (tc *TemplateContext) File() *File {
	if tc.Mode == ServiceMode {
		return tc.service.File
	}
	return tc.file
}

func (tc *TemplateContext) Service() *Service {
	if tc.Mode == ServiceMode {
		return tc.service
	}
	return nil
}

func (tc *TemplateContext) TemplateContext() *TemplateContext {
	return getContext(tc.File().Proto.GetName())
}

func (tc *TemplateContext) PragmaOnce() bool {
	return pragmaOnce(tc)
}
