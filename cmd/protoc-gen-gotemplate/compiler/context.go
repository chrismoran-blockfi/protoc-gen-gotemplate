package compiler

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strconv"
)

func (tc *TemplateContext) RenderImports() string {
	tc.impMu.Lock()
	defer tc.impMu.Unlock()

	imports := make(map[GoImportPath]GoPackageName)
	sorted := make([]string, 0)
	for importPath := range tc.addedImports {
		if _, ok := imports[importPath]; ok {
			continue
		}
		imports[importPath] = tc.goPackageName(importPath)
	}
	for k := range imports {
		sorted = append(sorted, string(k))
	}
	sort.Strings(sorted)
	buf := new(bytes.Buffer)
	buf.WriteString("import (")
	for _, skey := range sorted {
		importPath := GoImportPath(skey)
		packageName := imports[importPath]
		if string(packageName) == baseName(skey) {
			buf.WriteString(fmt.Sprintf("\n\t%s", importPath))
		} else {
			buf.WriteString(fmt.Sprintf("\n\t%s %s", packageName, importPath))
		}
	}
	buf.WriteString("\n)")
	buff, _ := format.Source(buf.Bytes())
	return string(buff)
}

func (tc *TemplateContext) goPackageName(importPath GoImportPath) GoPackageName {
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
	tc.impMu.Lock()
	defer tc.impMu.Unlock()

	importPath := GoImportPath(i)
	tc.addedImports[importPath] = true
	tc.usedPackages[importPath] = true
	return tc.goPackageName(importPath)
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
