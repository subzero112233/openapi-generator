package traverser

type API struct {
	Title       string
	Description string
	GoName      string
	JsName      string
	Version     string
	Contact     Contact
	Servers     []Server
	Methods     []Method
	Schemas     []Schema
	Consts      []Const
	RefDocs     map[string]API
	Specs       []Spec
}

type Spec struct {
	Doc    string
	Dir    string
	Title  string
	GoName string
	JsName string
}

func (api API) GetSchema(name string) Schema {
	for _, schema := range api.Schemas {
		if schema.GoName == name || schema.JsName == name {
			return schema
		}
	}

	return Schema{}
}

func (api API) IsConst(typeName string) bool {
	for _, c := range api.Consts {
		if c.Name == typeName {
			return true
		}
	}

	return false
}

func (api API) FindConst(constName string, constValue interface{}) ConstValue {
	for _, c := range api.Consts {
		if c.Name == constName {
			for _, v := range c.Values {
				if v.APIName == constValue.(string) {
					return v
				}
			}
		}
	}

	return ConstValue{}
}

type Contact struct {
	Name string
	URL  string
}

type Server struct {
	URL         string
	Name        string
	Description string
}

type Method struct {
	APIName          string
	GoName           string
	JsName           string
	Path             string
	FrmwkPath        string
	Summary          string
	Description      string
	HTTPMethod       string
	InputType        string
	OutputType       string
	InputInBody      bool
	SuccessfulStatus int
	Source           string
}

type Schema struct {
	APIName       string
	GoName        string
	JsName        string
	Description   string
	ErrorFormat   string
	Params        []Param
	OneOf         []string
	AnyOf         []string
	AllOf         []string
	Discriminator Discriminator
	Source        string
}

type Discriminator struct {
	APIName string
	GoName  string
	JsName  string
	Mapping map[string]string
}

func (schema Schema) GetParam(name string) Param {
	for _, p := range schema.Params {
		if p.APIName == name {
			return p
		}
	}

	return Param{}
}

type Param struct {
	APIName         string
	SpecName        string
	GoName          string
	JsName          string
	GoType          string
	JsType          string
	IsArray         bool
	ArrayItemGoType string
	ArrayItemJsType string
	Required        bool
	AllowEmpty      bool
	Description     string
	Deprecated      bool
	In              string
	Tags            string
	Default         interface{}
	MinItems        interface{}
	MaxItems        interface{}
	Minimum         interface{}
	Maximum         interface{}
	UniqueItems     bool
	MinLength       interface{}
	MaxLength       interface{}
	ValidURL        bool
	RequiredIf      RequiredIf
}

type RequiredIf struct {
	Needs string
	ToBe  interface{}
}

type Const struct {
	Name   string
	Values []ConstValue
	Source string
}

type ConstValue struct {
	APIName     string
	GoName      string
	JsName      string
	Description string
}

const (
	String   = "string"
	Byte     = "byte"
	Binary   = "binary"
	Date     = "date"
	DateTime = "date-time"
	Password = "password"
	Integer  = "integer"
	Int32    = "int32"
	Int64    = "int64"
	Uint64   = "uint64"
	Uint32   = "uint32"
	Uint16   = "uint16"
	Uint8    = "uint8"
	Number   = "number"
	Float    = "float"
	Boolean  = "boolean"
	Double   = "double"
	Array    = "array"
	Object   = "object"
)
