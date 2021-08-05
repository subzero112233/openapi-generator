package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/alecthomas/kong"

	"github.com/flosch/pongo2"

	"github.com/aquasecurity/openapi-generator/traverser"
)

var cli struct {
	// nolint: govet
	Parse struct {
		Pretty bool     `flag help:"pretty print"`
		Docs   []string `arg help:"document path(s)"`
	} `cmd help:"Parse specification, print JSON representation of API"`
	Generate struct {
		Template string   `arg help:"template path"`
		Docs     []string `arg help:"document path(s)"`
	} `cmd help:"Generate code"`
}

func main() {
	var err error
	ctx := kong.Parse(&cli)
	switch ctx.Command() {
	case "parse <docs>":
		err = parse()
	case "generate <template> <docs>":
		err = generate()
	default:
		err = fmt.Errorf("invalid/unimplemented command %s", ctx.Command())
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s\n", err)
		os.Exit(1)
	}
}

func mergeSpecs(docs []string) (api traverser.API, err error) {
	api = traverser.API{
		RefDocs: make(map[string]traverser.API),
	}

	for i, doc := range docs {
		this, err := loadAndParse(doc)
		if err != nil {
			return api, fmt.Errorf("failed parsing %s: %w", doc, err)
		}

		for i, schema := range this.Schemas {
			schema.Source = doc
			this.Schemas[i] = schema
		}
		for i, method := range this.Methods {
			method.Source = doc
			this.Methods[i] = method
		}
		for i, c := range this.Consts {
			c.Source = doc
			this.Consts[i] = c
		}

		if i == 0 {
			api.Title = this.Title
			api.Description = this.Description
			api.GoName = this.GoName
			api.JsName = this.JsName
			api.Version = this.Version
			api.Contact = this.Contact
			api.Servers = append(api.Servers, this.Servers...)
		}

		api.Methods = append(api.Methods, this.Methods...)
		api.Schemas = append(api.Schemas, this.Schemas...)
		api.Consts = append(api.Consts, this.Consts...)
		for key, val := range this.RefDocs {
			api.RefDocs[key] = val
		}

		api.Specs = append(api.Specs, traverser.Spec{
			Doc:    doc,
			Dir:    filepath.Base(filepath.Dir(doc)),
			Title:  this.Title,
			GoName: this.GoName,
			JsName: this.JsName,
		})
	}

	sort.Slice(api.Specs, func(i, j int) bool {
		return api.Specs[i].Dir < api.Specs[j].Dir
	})

	return api, nil
}

func parse() (err error) {
	// load and parse the API specification
	api, err := mergeSpecs(cli.Parse.Docs)
	if err != nil {
		return err
	}

	// echo API to stdout as JSON
	enc := json.NewEncoder(os.Stdout)
	if cli.Parse.Pretty {
		enc.SetIndent("", "  ")
	}
	enc.Encode(api)

	return nil
}

func generate() (err error) {
	// load and parse the API specification
	api, err := mergeSpecs(cli.Generate.Docs)
	if err != nil {
		return err
	}

	// load the template
	pongo2.SetAutoescape(false)
	tmpl, err := pongo2.FromFile(cli.Generate.Template)
	if err != nil {
		return fmt.Errorf("failed loading template: %w", err)
	}

	// render the template to standard output
	err = tmpl.ExecuteWriter(pongo2.Context{
		"api":             api,
		"comment":         comment,
		"wrapped_comment": wrappedComment,
	}, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed generating code: %w", err)
	}

	return nil
}

func loadAndParse(path string) (api traverser.API, err error) {
	// load the OpenAPI document
	var doc traverser.Map
	if strings.HasSuffix(path, ".yaml") {
		doc, err = traverser.LoadYAML(path)
	} else if strings.HasSuffix(path, ".json") {
		doc, err = traverser.LoadJSON(path)
	}
	if err != nil {
		return api, fmt.Errorf("failed loading spec: %w", err)
	}

	// parse the OpenAPI document
	return traverser.ParseDoc(doc)
}

func comment(text string) string {
	return strings.Replace(
		strings.Replace(
			strings.TrimSuffix(text, "\n"),
			"\n", " ", -1,
		),
		"\t", "", -1,
	)
}

func wrappedComment(lineLength int, indent string, args ...string) string {
	desc := strings.Join(args, "")
	desc = strings.Replace(strings.TrimSuffix(desc, "\n"), "\n", " ", -1)
	textLength := lineLength - len(indent) - 3
	var lines []string
	for len(desc) > textLength {
		lastRune := []rune(desc[:textLength])[textLength-1]
		nextRune := []rune(desc[:textLength+1])[textLength]
		if !unicode.IsSpace(lastRune) && !unicode.IsSpace(nextRune) {
			// we're gonna cut the text off mid-word
			desc = fmt.Sprintf("%s-%s", desc[:textLength-1], desc[textLength-1:])
		}
		lines = append(lines, fmt.Sprintf("%s// %s", indent, desc[:textLength]))
		desc = desc[textLength:]
	}
	if len(desc) > 0 {
		lines = append(lines, fmt.Sprintf("%s// %s", indent, desc))
	}
	return strings.Join(lines, "\n")
}
