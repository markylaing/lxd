package main

import (
	"strings"
	"text/template"
	"unicode"
)

// Spec defines how an inspected struct maps to the database.
type Spec struct {
	Reference          *Spec
	ReferenceFieldName string
	StructName         string
	TableName          string
	Fields             []FieldSpec
	Joins              []string
}

func (s *Spec) hasPrimaryKey() bool {
	for _, f := range s.Fields {
		if f.Primary {
			return true
		}
	}

	return false
}

// unqualifiedColumnName returns the column name for the field spec without the `<table_name>.` prepended and boolean
// indicating if the column is defined on the spec table (e.g. it is not joined or coalesced). If the column is not
// defined on the spec table then an empty string is returned.
func (s *Spec) unqualifiedColumnName(field FieldSpec) (string, bool) {
	col, ok := strings.CutPrefix(field.ColumnName, s.TableName+".")
	if !ok {
		return "", false
	}

	return col, true
}

// FieldSpec is a simple mapping of struct field name to database column name.
type FieldSpec struct {
	FieldName  string
	ColumnName string
	SkipCreate bool
	SkipUpdate bool
	Primary    bool
}

// templateContext returns the values used for rendering the template.
func (s *Spec) templateContext() map[string]any {
	return map[string]any{
		"receiver":             string(s.receiver()),
		"structName":           s.StructName,
		"tableName":            s.TableName,
		"backtick":             "`",
		"defaultSelectColumns": s.defaultSelectColumns(),
		"joins":                s.joins(),
		"scanArgs":             s.scanArgs(),
		"defaultCreateColumns": s.defaultCreateColumns(),
		"defaultUpdateColumns": s.defaultUpdateColumns(),
		"primaryKeyColumns":    s.primaryKeyColumns(),
		"values":               s.values(),
		"genAPIName":           s.Reference != nil,
		"apiName":              s.apiName(),
	}
}

var specSelectTemplate = template.Must(template.New("select").Parse(`
// TableName returns the table name for [{{ .structName }}] entities.
func ({{ .receiver }} {{ .structName }}) TableName() string {
	return "{{ .tableName }}"
}
{{ if .genAPIName }}
// APIName implements [query.APINamer] for API friendly error messages.
func ({{ .receiver }} {{ .structName }}) APIName() string {
	return {{ .apiName }}
}
{{ end }}
// DefaultSelectColumns returns a slice of default column names for [{{ .structName }}] queries.
func ({{ .receiver }} {{ .structName }}) DefaultSelectColumns() []string {
	return {{ .defaultSelectColumns }}
}

// Joins returns a slice of join expressions for [{{ .structName }}].
func ({{ .receiver }} {{ .structName }}) Joins() []string {
	return {{ .joins }}
}

// ScanArgs implements [query.ScanArger] for [{{ .structName }}].
// This returns references to struct fields in definition order.
func ({{ .receiver }} *{{ .structName }}) ScanArgs(columns ...string) ([]any, error) {
	{{ .scanArgs }}
}
`))

var specExecTemplate = template.Must(template.New("exec").Parse(`
// PrimaryKeyColumns returns the column names for the primary key of a [{{ .structName }}] entity used during an update.
func ({{ .receiver }} {{ .structName }}) PrimaryKeyColumns() []string {
	return {{ .primaryKeyColumns }}
}

// DefaultCreateColumns returns a list of default column names used for [{{ .structName }}] creation.
func ({{ .receiver }} {{ .structName }}) DefaultCreateColumns() []string {
	return {{ .defaultCreateColumns }}
}

// DefaultUpdateColumns returns a list of default column names used for [{{ .structName }}] mutation.
func ({{ .receiver }} {{ .structName }}) DefaultUpdateColumns() []string {
	return {{ .defaultUpdateColumns }}
}

// Values returns the fields from [{{ .structName }}] in order of the specified columns.
func ({{ .receiver }} {{ .structName }}) Values(columns ...string) ([]any, error) {
	{{ .values }}
}
`))

func (s *Spec) apiName() string {
	if s.Reference == nil {
		return ""
	}

	return string(s.receiver()) + "." + s.ReferenceFieldName + ".APIName()"
}

func (s *Spec) receiver() rune {
	return unicode.ToLower(rune(s.StructName[0]))
}

func (s *Spec) joins() string {
	if len(s.Joins) == 0 {
		return "[]string{}"
	}

	return "[]string{\n\t\t\"" + strings.Join(s.Joins, "\",\n\t\t\"") + "\",\n\t}"
}

func (s *Spec) defaultSelectColumns() string {
	cols := make([]string, 0, len(s.Fields))
	if s.Reference != nil {
		cols = make([]string, 0, len(s.Fields)+len(s.Reference.Fields))
		for _, f := range s.Reference.Fields {
			cols = append(cols, f.ColumnName)
		}
	}

	for _, f := range s.Fields {
		cols = append(cols, f.ColumnName)
	}

	return "[]string{\n\t\t\"" + strings.Join(cols, "\",\n\t\t\"") + "\",\n\t}"
}

func (s *Spec) defaultCreateColumns() string {
	cols := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		unqualifiedColName, ok := s.unqualifiedColumnName(f)
		if !ok {
			continue
		}

		if unqualifiedColName == columnID || f.SkipCreate {
			continue
		}

		cols = append(cols, unqualifiedColName)
	}

	return "[]string{\n\t\t\"" + strings.Join(cols, "\",\n\t\t\"") + "\",\n\t}"
}

func (s *Spec) defaultUpdateColumns() string {
	cols := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		unqualifiedColName, ok := s.unqualifiedColumnName(f)
		if !ok {
			continue
		}

		if unqualifiedColName == columnID || f.SkipUpdate {
			continue
		}

		cols = append(cols, unqualifiedColName)
	}

	return "[]string{\n\t\t\"" + strings.Join(cols, "\",\n\t\t\"") + "\",\n\t}"
}

func (s *Spec) primaryKeyColumns() string {
	cols := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		if f.Primary {
			cols = append(cols, strings.TrimPrefix(f.ColumnName, s.TableName+"."))
		}
	}

	return `[]string{"` + strings.Join(cols, `", "`) + `"}`
}

func (s *Spec) scanArgs() string {
	var b strings.Builder

	b.WriteString(`args := make([]any, 0, len(columns))
	for _, column := range columns {
		switch column {`)

	if s.Reference != nil {
		for _, f := range s.Reference.Fields {
			b.WriteString("\n\t\tcase\"")
			b.WriteString(f.ColumnName)
			b.WriteString("\":\n\t\t\t")
			b.WriteString("args = append(args, &")
			b.WriteString(string(s.receiver()))
			b.WriteString(".")
			b.WriteString(s.ReferenceFieldName)
			b.WriteString(".")
			b.WriteString(f.FieldName)
			b.WriteString(")")
		}
	}

	for _, f := range s.Fields {
		b.WriteString("\n\t\tcase \"")
		b.WriteString(f.ColumnName)
		b.WriteString("\":\n\t\t\t")
		b.WriteString("args = append(args, &")
		b.WriteString(string(s.receiver()))
		b.WriteString(".")
		b.WriteString(f.FieldName)
		b.WriteString(")")

		if s.Reference == nil {
			b.WriteString("\n\t\tcase \"")
			b.WriteString(strings.TrimPrefix(f.ColumnName, s.TableName+"."))
			b.WriteString("\":\n\t\t\t")
			b.WriteString("args = append(args, &")
			b.WriteString(string(s.receiver()))
			b.WriteString(".")
			b.WriteString(f.FieldName)
			b.WriteString(")")
		}
	}

	b.WriteString("\n\t\tdefault:\n\t\t\treturn nil, fmt.Errorf(\"Unknown column %q for table %q\", column, ")
	b.WriteString(string(s.receiver()))
	b.WriteString(".TableName())\n\t\t}\n\t}\n\n\treturn args, nil")

	return b.String()
}

func (s *Spec) values() string {
	var b strings.Builder

	b.WriteString(`values := make([]any, 0, len(columns))
	for _, column := range columns {
		switch column {`)

	for _, f := range s.Fields {
		b.WriteString("\n\t\tcase \"")
		b.WriteString(strings.TrimPrefix(f.ColumnName, s.TableName+"."))
		b.WriteString("\":\n\t\t\t")
		b.WriteString("values = append(values, ")
		b.WriteString(string(s.receiver()))
		b.WriteString(".")
		b.WriteString(f.FieldName)
		b.WriteString(")")
	}

	b.WriteString("\n\t\tdefault:\n\t\t\treturn nil, fmt.Errorf(\"Unknown column %q for table %q\", column, ")
	b.WriteString(string(s.receiver()))
	b.WriteString(".TableName())\n\t\t}\n\t}\n\n\treturn values, nil")

	return b.String()
}