package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"

	"github.com/upamune/tla-go/checker"
)

// This is a metadata projection, not a complete SANY semantic graph.
type sanyMetadataXML struct {
	XMLName xml.Name    `xml:"modules"`
	Root    string      `xml:"RootModule"`
	Entries []sanyEntry `xml:"context>entry"`
}
type sanyEntry struct {
	UID         int              `xml:"UID"`
	Declaration *sanyDeclaration `xml:"OpDeclNode,omitempty"`
	Definition  *sanyDefinition  `xml:"UserDefinedOpKind,omitempty"`
	Module      *sanyModule      `xml:"ModuleNode,omitempty"`
}
type sanyDeclaration struct {
	Name  string `xml:"uniquename"`
	Kind  int    `xml:"kind"`
	Level int    `xml:"level"`
}
type sanyDefinition struct {
	Name  string `xml:"uniquename"`
	Level int    `xml:"level"`
}
type sanyModule struct {
	Name         string    `xml:"uniquename"`
	Declarations []sanyRef `xml:"OpDeclNodeRef"`
	Definitions  []sanyRef `xml:"UserDefinedOpKindRef"`
}
type sanyRef struct {
	UID int `xml:"UID"`
}

func runSANYMetadata(ctx context.Context, opts options, out, errout io.Writer) int {
	if len(opts.paths) != 1 {
		fmt.Fprintln(errout, "-sany-metadata requires exactly one module")
		return 255
	}
	opts.path = opts.paths[0]
	path, _, err := inputPaths(opts)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 255
	}
	module, code := loadModule(path, opts.libraryPaths, errout)
	if code != 0 {
		return code
	}
	metadata, err := checker.AnalyzeMetadata(ctx, module, directoryLoader(filepath.Dir(path), opts.libraryPaths))
	if err != nil {
		return reportError(errout, err)
	}
	doc := sanyMetadataDocument(metadata)
	data, err := xml.Marshal(doc)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 255
	}
	if _, err := fmt.Fprint(out, xml.Header, string(data), "\n"); err != nil {
		fmt.Fprintln(errout, err)
		return 255
	}
	return 0
}

func sanyMetadataDocument(metadata checker.SemanticMetadata) sanyMetadataXML {
	doc := sanyMetadataXML{Root: metadata.Module}
	module := sanyModule{Name: metadata.Module}
	for _, name := range metadata.Variables {
		uid := len(doc.Entries) + 1
		doc.Entries = append(doc.Entries, sanyEntry{UID: uid, Declaration: &sanyDeclaration{Name: name, Kind: 3, Level: 1}})
		module.Declarations = append(module.Declarations, sanyRef{UID: uid})
	}
	for _, name := range metadata.Constants {
		uid := len(doc.Entries) + 1
		doc.Entries = append(doc.Entries, sanyEntry{UID: uid, Declaration: &sanyDeclaration{Name: name, Kind: 2}})
		module.Declarations = append(module.Declarations, sanyRef{UID: uid})
	}
	for _, d := range metadata.Definitions {
		uid := len(doc.Entries) + 1
		doc.Entries = append(doc.Entries, sanyEntry{UID: uid, Definition: &sanyDefinition{Name: d.Name, Level: d.Level}})
		module.Definitions = append(module.Definitions, sanyRef{UID: uid})
	}
	doc.Entries = append(doc.Entries, sanyEntry{UID: len(doc.Entries) + 1, Module: &module})
	return doc
}
