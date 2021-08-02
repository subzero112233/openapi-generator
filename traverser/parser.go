package traverser

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type parser struct {
	api    *API
	doc    Map
	consts map[string]Const
}

func ParseDoc(doc Map) (api API, err error) {
	api.Title = doc.Str("info", "title")
	api.Description = doc.Str("info", "description")
	api.GoName = fmt.Sprintf("%s", strings.Replace(api.Title, " ", "", -1))
	api.JsName = fmt.Sprintf("%s", strings.ToLower(api.GoName))
	api.Version = doc.Str("info", "version")
	api.Contact.Name = doc.Str("info", "contact", "name")
	api.Contact.URL = doc.Str("info", "contact", "url")
	api.RefDocs = make(map[string]API)

	p := &parser{
		api:    &api,
		doc:    doc,
		consts: make(map[string]Const),
	}

	for _, fn := range []func() error{
		p.parseServers,
		p.parseSchemas,
		p.parsePaths,
		p.parseEnums,
	} {
		err = fn()
		if err != nil {
			return api, err
		}
	}

	return api, nil
}

func (p *parser) parseServers() error {
	if len(p.doc.Slice("servers")) > 0 {
		for _, server := range p.doc.Slice("servers") {
			p.api.Servers = append(p.api.Servers, Server{
				URL:         server.Str("url"),
				Name:        strings.Title(server.Str("x-name")),
				Description: server.Str("description"),
			})
		}
	}

	return nil
}

func (p *parser) parseSchemas() error {
	for _, name := range p.doc.Keys("components", "schemas") {
		s, _ := p.doc.Get("components", "schemas", name)
		switch s.Str("type") {
		case Object:
			p.parseSchema(name, s)
		case String:
			p.parseConst(name, s)
		}
	}

	return nil
}

func (p *parser) parseSchema(name string, s Any) {
	required := make(map[string]bool)
	for _, param := range s.Slice("required") {
		required[param.Str()] = true
	}

	schema := Schema{
		APIName:     name,
		GoName:      name,
		JsName:      name,
		Description: s.Str("description"),
		ErrorFormat: s.Str("x-go-error"),
	}

	for _, propName := range s.Keys("properties") {
		prop, _ := s.Get("properties", propName)

		p.parseConst(propName, prop)

		schema.Params = append(schema.Params, parseProp(
			propName,
			prop,
			required[propName],
		))
	}

	for _, ref := range []struct {
		name   string
		target *[]string
	}{
		{"anyOf", &schema.AnyOf},
		{"allOf", &schema.AllOf},
		{"oneOf", &schema.OneOf},
	} {
		for _, refSchem := range s.Slice(ref.name) {
			*ref.target = append(
				*ref.target,
				strings.TrimPrefix(refSchem.Str("$ref"), "#/components/schemas/"),
			)
		}
	}

	if disc, ok := s.Get("discriminator"); ok {
		mapping := make(map[string]string)

		if _, ok := disc.Get("mapping"); ok {
			for _, val := range disc.Keys("mapping") {
				mapping[val] = disc.Str("mapping", val)
			}
		}

		apiName := disc.Str("propertyName")

		schema.Discriminator = Discriminator{
			APIName: apiName,
			GoName:  goName(apiName),
			JsName:  apiName,
			Mapping: mapping,
		}
	}

	p.api.Schemas = append(p.api.Schemas, schema)
}

func (p *parser) parsePaths() error {
	for _, path := range p.doc.Keys("paths") {
		subDoc, _ := p.doc.Get("paths", path)
		err := p.parsePath(path, subDoc)
		if err != nil {
			return fmt.Errorf("failed parsing path %s: %w", path, err)
		}
	}

	return nil
}

var (
	externalRefRegex = regexp.MustCompile(`^([^#]+)?(#.+)$`)
	pathParamsRegex  = regexp.MustCompile(`{([^}]+)}`)
)

func (p *parser) parsePath(path string, subDoc Any) (err error) {
	if ref := subDoc.Str("$ref"); ref != "" {
		// we need to parse the referenced document
		matches := externalRefRegex.FindStringSubmatch(ref)
		if len(matches) != 3 {
			return fmt.Errorf("invalid reference format %q for path %s", ref, path)
		}

		if _, ok := p.api.RefDocs[matches[1]]; !ok {
			var refDoc Map
			switch {
			case strings.HasPrefix(matches[1], ".json"):
				refDoc, err = LoadJSON(matches[1])
			default:
				refDoc, err = LoadYAML(matches[1])
			}
			if err != nil {
				return fmt.Errorf("failed loading referenced file %s: %w", matches[1], err)
			}

			refAPI, err := ParseDoc(refDoc)
			if err != nil {
				return fmt.Errorf("failed parsing referenced file %s: %w", matches[1], err)
			}

			p.api.RefDocs[matches[1]] = refAPI
			p.api.Schemas = append(p.api.Schemas, refAPI.Schemas...)
			p.api.Methods = append(p.api.Methods, refAPI.Methods...)
			p.api.Consts = append(p.api.Consts, refAPI.Consts...)
		}

		return nil
	}

	commonParams := subDoc.Slice("parameters")
	for _, method := range subDoc.Keys() {
		m, _ := subDoc.Get(method)
		if m.Str("operationId") == "" {
			continue
		}

		var apiMethod Method
		apiMethod.APIName = m.Str("operationId")
		apiMethod.GoName = goName(apiMethod.APIName)
		apiMethod.JsName = jsMethod(apiMethod.APIName)
		apiMethod.Path = path
		apiMethod.FrmwkPath = pathParamsRegex.ReplaceAllString(path, ":$1")
		apiMethod.HTTPMethod = strings.ToUpper(method)
		apiMethod.Summary = m.Str("summary")
		apiMethod.Description = m.Str("description")
		apiMethod.InputType = inputType(m)
		apiMethod.OutputType = outputType(m)
		_, apiMethod.InputInBody = m.Get("requestBody")

		// what is the successful status code for this method?
		for _, status := range m.Keys("responses") {
			if strings.HasPrefix(status, "2") {
				apiMethod.SuccessfulStatus, _ = strconv.Atoi(status)
			}
		}

		p.api.Methods = append(p.api.Methods, apiMethod)

		if apiMethod.InputInBody {
			// if there are common params, we need to find this schema and
			// create an extension of it that includes these params
			if len(commonParams) > 0 {
				for i, schema := range p.api.Schemas {
					if schema.GoName == apiMethod.InputType {
						for _, param := range commonParams {
							schema.Params = append(schema.Params, parseParam(param, true))
						}
					}

					p.api.Schemas[i] = schema
				}
			}
			continue
		}

		schema := Schema{
			APIName: apiMethod.APIName,
			GoName:  apiMethod.InputType,
			JsName:  apiMethod.InputType,
		}

		params := append(commonParams, m.Slice("parameters")...)
		for _, p := range params {
			schema.Params = append(schema.Params, parseParam(p, false))
		}

		p.api.Schemas = append(p.api.Schemas, schema)
	}

	return nil
}

func (p *parser) parseEnums() error {
	constNames := make([]string, len(p.consts))
	var i int
	for name := range p.consts {
		constNames[i] = name
		i++
	}

	sort.Strings(constNames)

	for _, name := range constNames {
		c := p.consts[name]

		for i, val := range c.Values {
			if val.GoName == "" {
				val.GoName = fmt.Sprintf("%s%s", name, goName(val.APIName))
			}
			if val.JsName == "" {
				val.JsName = jsConst(val.APIName)
			}

			c.Values[i] = val
		}

		p.api.Consts = append(p.api.Consts, c)
	}

	return nil
}

func goName(name string) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(
				strings.Title(strings.Replace(
					strings.Replace(name, "_", " ", -1),
					"-", " ", -1),
				),
				" ", "", -1,
			), "Url", "URL", -1,
		), "Id", "ID", -1,
	)
}

func parseParam(schema Any, noJSON bool) Param {
	param := Param{
		APIName:     schema.Str("name"),
		SpecName:    "{" + schema.Str("name") + "}",
		Required:    schema.Bool("required"),
		AllowEmpty:  schema.Bool("allowEmptyValue"),
		Description: schema.Str("description"),
		Deprecated:  schema.Bool("deprecated"),
		In:          schema.Str("in"),
		IsArray:     schema.Str("schema", "type") == Array,
	}

	param.GoName = goName(param.APIName)
	param.JsName = param.APIName

	var tags []string
	if noJSON {
		tags = append(tags, `json:"-"`)
	} else if !param.Required {
		tags = append(tags, fmt.Sprintf(`json:"%s,omitempty"`, param.APIName))
	} else {
		tags = append(tags, fmt.Sprintf(`json:"%s"`, param.APIName))
	}

	tags = append(tags, fmt.Sprintf(`lambda:"%s.%s"`, param.In, param.APIName))

	param.Tags = fmt.Sprintf("`%s`", strings.Join(tags, " "))

	schema, _ = schema.Get("schema")
	param.GoType, param.ArrayItemGoType = goType(param.APIName, schema, param.Required)
	param.JsType, param.ArrayItemJsType = jsType(param.APIName, schema, param.Required)
	parseParamValidations(&param, schema)

	return param
}

func parseProp(name string, schema Any, req bool) Param {
	param := Param{
		APIName:     name,
		Required:    req,
		Description: schema.Str("description"),
		Deprecated:  schema.Bool("deprecated"),
		IsArray:     schema.Str("type") == Array,
	}

	var tags []string
	if !param.Required {
		tags = append(tags, fmt.Sprintf(`json:"%s,omitempty"`, param.APIName))
	} else {
		tags = append(tags, fmt.Sprintf(`json:"%s"`, param.APIName))
	}

	param.Tags = fmt.Sprintf("`%s`", strings.Join(tags, " "))
	param.GoName = goName(param.APIName)
	param.JsName = param.APIName
	if schema.Bool("x-delay") {
		param.GoType = "json.RawMessage"
	} else if schema.Bool("additionalProperties") {
		param.GoType = "map[string]interface{}"
	} else {
		param.GoType, param.ArrayItemGoType = goType(param.APIName, schema, param.Required)
	}
	param.JsType, param.ArrayItemJsType = jsType(param.APIName, schema, param.Required)
	parseParamValidations(&param, schema)

	return param
}

func parseParamValidations(param *Param, schema Any) {
	switch param.GoType {
	case String:
		if def, ok := schema.Get("default"); ok {
			param.Default = def.Str()
		}
		if min, ok := schema.Get("minLength"); ok {
			param.MinLength = min.Int64()
		}
		if max, ok := schema.Get("maxLength"); ok {
			param.MaxLength = max.Int64()
		}
	case Int64, Int32, Uint64, Uint32, Uint16, Uint8:
		if def, ok := schema.Get("default"); ok {
			param.Default = def.Int64()
		}
		if min, ok := schema.Get("minimum"); ok {
			param.Minimum = min.Int64()
		}
		if max, ok := schema.Get("maximum"); ok {
			param.Maximum = max.Int64()
		}
	}

	if reqIf, ok := schema.Get("x-required-if"); ok {
		toBe, _ := reqIf.Get("to_be")
		param.RequiredIf = RequiredIf{
			Needs: reqIf.Str("needs"),
			ToBe:  toBe.data,
		}
	}

	param.ValidURL = schema.Bool("x-valid-url")
}

func goType(name string, schema Any, req bool) (typeName, arrayTypeName string) {
	if schema.Str("$ref") != "" {
		matches := externalRefRegex.FindStringSubmatch(schema.Str("$ref"))
		if len(matches) != 3 {
			panic("Invalid $ref format for " + name)
		}

		typeName := strings.TrimPrefix(matches[2], "#/components/schemas/")
		if matches[1] != "" {
			typeName = fmt.Sprintf(
				"%s.%s",
				filepath.Base(filepath.Dir(matches[1])), typeName,
			)
		}

		if req || (schema.Str("type") == String && schema.Str("format") == "") {
			return typeName, arrayTypeName
		} else {
			return fmt.Sprintf("*%s", typeName), arrayTypeName
		}
	}

	switch schema.Str("type") {
	case String:
		switch schema.Str("format") {
		case Date, DateTime:
			if req {
				return "time.Time", ""
			} else {
				return "*time.Time", ""
			}
		case Password:
			return "[]byte", "byte"
		default:
			if len(schema.Slice("enum")) > 0 {
				enumName := schema.Str("x-enum-name")
				if enumName != "" {
					return enumName, ""
				} else {
					return goName(name), ""
				}
			} else {
				return "string", ""
			}
		}
	case Integer:
		return schema.Str("format"), ""
	case Number:
		switch schema.Str("format") {
		case Float:
			return "float32", ""
		default:
			return "float64", ""
		}
	case Boolean:
		if req {
			return "bool", ""
		} else {
			return "*bool", ""
		}
	case Object:
		return "map[string]interface{}", ""
	case Array:
		if items, ok := schema.Get("items"); ok {
			arrayTypeName, _ = goType(name, items, true)
			return fmt.Sprintf("[]%s", arrayTypeName), arrayTypeName
		}
		return "[]interface{}", "interface{}"
	}

	return "interface{}", ""
}

func jsType(name string, schema Any, req bool) (typeName string, arrayTypeName string) {
	if schema.Str("$ref") != "" {
		typeName = strings.TrimPrefix(schema.Str("$ref"), "#/components/schemas/")
		if req || (schema.Str("type") == String && schema.Str("format") == "") {
			return typeName, arrayTypeName
		} else {
			return fmt.Sprintf("?%s", typeName), arrayTypeName
		}
	}

	switch schema.Str("type") {
	case String:
		switch schema.Str("format") {
		case Date, DateTime:
			if req {
				return "Date", ""
			} else {
				return "?Date", ""
			}
		case Password:
			return "string", ""
		default:
			if len(schema.Slice("enum")) > 0 {
				enumName := schema.Str("x-enum-name")
				if enumName != "" {
					return enumName, ""
				} else {
					return goName(name), ""
				}
			} else {
				return "string", ""
			}
		}
	case Integer, Number:
		return "number", ""
		return "number", ""
	case Boolean:
		return "boolean", ""
	case Array:
		schema := strings.TrimPrefix(schema.Str("items", "$ref"), "#/components/schemas/")
		return fmt.Sprintf("Array<%s>", schema), schema
	}

	return "any", ""
}

func inputType(m Any) string {
	title := goName(m.Str("operationId"))
	inputName := strings.TrimPrefix(
		m.Str("requestBody", "content", "application/json", "schema", "$ref"),
		"#/components/schemas/",
	)
	if inputName == "" {
		inputName = title + "Input"
	}

	return inputName
}

func outputType(m Any) string {
	title := goName(m.Str("operationId"))
	outputName := title + "Output"
	for _, status := range m.Keys("responses") {
		if strings.HasPrefix(status, "2") {
			outputName = strings.TrimPrefix(
				m.Str("responses", status, "content", "application/json", "schema", "$ref"),
				"#/components/schemas/",
			)
		}
	}

	return outputName
}

func (p *parser) parseConst(propName string, prop Any) {
	enum := prop.Slice("enum")
	if len(enum) == 0 {
		return
	}

	var constName string
	var constVals []ConstValue

	customEnum, ok := prop.Get("custom-enum")
	if ok {
		constName = customEnum.Str("name")

		opts := customEnum.Keys("options")
		constVals = make([]ConstValue, len(opts))
		for i, opt := range opts {
			constVals[i] = ConstValue{
				APIName:     opt,
				GoName:      customEnum.Str("options", opt, "go_name"),
				JsName:      customEnum.Str("options", opt, "js_name"),
				Description: customEnum.Str("options", opt, "description"),
			}
		}
	} else {
		constVals = make([]ConstValue, len(enum))
		for i, val := range enum {
			constVals[i] = ConstValue{
				APIName: val.Str(),
			}
		}
	}

	constName = goName(propName)

	p.consts[constName] = Const{
		Name:   constName,
		Values: constVals,
	}
}

func jsConst(text string) string {
	return strings.Replace(
		strings.Replace(
			strings.ToUpper(text),
			" ", "_", -1,
		), "-", "", -1,
	)
}

func jsMethod(text string) string {
	return strings.Replace(text, "-", "_", -1)
}
