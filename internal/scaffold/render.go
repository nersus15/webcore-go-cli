package scaffold

import (
	"bytes"
	"strings"
	"text/template"
)

var templateFuncs = template.FuncMap{
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
}

// render mengisi template Go `text/template` dengan data map.
func render(tmpl string, data any) ([]byte, error) {
	t, err := template.New("tmpl").Funcs(templateFuncs).Parse(tmpl)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
