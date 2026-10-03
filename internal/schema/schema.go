// Package schema validates litefaas.yaml / stack.yaml against the embedded JSON Schema.
// The validator implements the subset this schema uses: type, enum, required,
// properties, additionalProperties, items, minimum, and maximum.
// A remote $schema URL is never fetched.
package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed litefaas.schema.json
var embedded []byte

const CanonicalID = "https://raw.githubusercontent.com/wolvever/litefaas/main/schema/litefaas.schema.json"

type Diagnostic struct {
	Path     string
	Severity string // error | warning
	Message  string
}

func (d Diagnostic) String() string {
	p := d.Path
	if p == "" {
		p = "(root)"
	}
	return d.Severity + ": " + p + ": " + d.Message
}

type node struct {
	Type             string           `json:"type"`
	Required         []string         `json:"required"`
	Properties       map[string]*node `json:"properties"`
	Additional       json.RawMessage  `json:"additionalProperties"`
	Enum             []any            `json:"enum"`
	Items            *node            `json:"items"`
	Minimum          *float64         `json:"minimum"`
	Maximum          *float64         `json:"maximum"`
	additionalFalse  bool
	additionalSchema *node
}

func Embedded() []byte { return embedded }

func load() (*node, error) {
	var n node
	if err := json.Unmarshal(embedded, &n); err != nil {
		return nil, err
	}
	n.prepare()
	return &n, nil
}

func (n *node) prepare() {
	if n == nil {
		return
	}
	if len(n.Additional) > 0 {
		var b bool
		if json.Unmarshal(n.Additional, &b) == nil {
			n.additionalFalse = !b
		} else {
			var child node
			if json.Unmarshal(n.Additional, &child) == nil {
				child.prepare()
				n.additionalSchema = &child
			}
		}
	}
	for _, c := range n.Properties {
		c.prepare()
	}
	if n.Items != nil {
		n.Items.prepare()
	}
}

// ValidatePath validates litefaas.yaml or stack.yaml at path (file or directory).
// A missing manifest returns no diagnostics (caller may be fingerprint-only).
func ValidatePath(path string) ([]Diagnostic, error) {
	if path == "" {
		path = "."
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		for _, name := range []string{"litefaas.yaml", "stack.yaml", "stack.yml"} {
			p := filepath.Join(path, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return ValidateFile(p)
			}
		}
		return nil, nil
	}
	return ValidateFile(path)
}

// ValidateFile validates one manifest file.
func ValidateFile(path string) ([]Diagnostic, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		return []Diagnostic{{Path: path, Severity: "error", Message: "yaml: " + err.Error()}}, nil
	}
	sch, err := load()
	if err != nil {
		return nil, err
	}
	base := filepath.Base(path)
	if base == "stack.yaml" || base == "stack.yml" || isStackDoc(v) {
		return validateStack(v, sch), nil
	}
	var out []Diagnostic
	validate(v, sch, "", &out)
	noteSchema(v, "", &out)
	return out, nil
}

func isStackDoc(v any) bool {
	m, ok := asMap(v)
	if !ok {
		return false
	}
	_, has := m["services"]
	_, runtime := m["runtime"]
	return has && !runtime
}

func validateStack(v any, svc *node) []Diagnostic {
	var out []Diagnostic
	m, ok := asMap(v)
	if !ok {
		return []Diagnostic{{Severity: "error", Message: "stack.yaml must be a mapping"}}
	}
	noteSchema(v, "", &out)
	for k := range m {
		if k == "services" || k == "$schema" {
			continue
		}
		out = append(out, Diagnostic{Path: k, Severity: "warning", Message: "unknown key"})
	}
	services, ok := m["services"]
	if !ok {
		out = append(out, Diagnostic{Path: "services", Severity: "error", Message: "required"})
		return out
	}
	switch sv := services.(type) {
	case map[string]any:
		for name, body := range sv {
			bm, ok := asMap(body)
			if !ok {
				out = append(out, Diagnostic{Path: "services." + name, Severity: "error", Message: "must be a mapping"})
				continue
			}
			if _, has := bm["name"]; !has {
				bm["name"] = name
			}
			validate(bm, svc, "services."+name, &out)
			noteSchema(bm, "services."+name, &out)
		}
	case []any:
		for i, body := range sv {
			path := fmt.Sprintf("services[%d]", i)
			validate(body, svc, path, &out)
			noteSchema(body, path, &out)
		}
	default:
		out = append(out, Diagnostic{Path: "services", Severity: "error", Message: "must be a map or list"})
	}
	return out
}

func noteSchema(v any, path string, out *[]Diagnostic) {
	m, ok := asMap(v)
	if !ok {
		return
	}
	raw, ok := m["$schema"]
	if !ok {
		return
	}
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" || s == CanonicalID {
		return
	}
	p := "$schema"
	if path != "" {
		p = path + ".$schema"
	}
	*out = append(*out, Diagnostic{Path: p, Severity: "warning", Message: "external $schema is not fetched; validating with the embedded schema"})
}

func validate(v any, sch *node, path string, out *[]Diagnostic) {
	if sch == nil {
		return
	}
	if len(sch.Enum) > 0 && !enumMatch(v, sch.Enum) {
		*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: "must be one of " + enumList(sch.Enum)})
	}
	switch sch.Type {
	case "object":
		m, ok := asMap(v)
		if !ok {
			*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: "must be an object"})
			return
		}
		for _, req := range sch.Required {
			if _, ok := m[req]; !ok {
				p := req
				if path != "" {
					p = path + "." + req
				}
				*out = append(*out, Diagnostic{Path: p, Severity: "error", Message: "required"})
			}
		}
		for k, child := range m {
			sub := sch.Properties[k]
			p := k
			if path != "" {
				p = path + "." + k
			}
			if sub == nil {
				if sch.additionalSchema != nil {
					validate(child, sch.additionalSchema, p, out)
					continue
				}
				if sch.additionalFalse {
					*out = append(*out, Diagnostic{Path: p, Severity: "warning", Message: "unknown key"})
				}
				continue
			}
			validate(child, sub, p, out)
		}
	case "array":
		arr, ok := asSlice(v)
		if !ok {
			*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: "must be an array"})
			return
		}
		if sch.Items != nil {
			for i, item := range arr {
				validate(item, sch.Items, fmt.Sprintf("%s[%d]", path, i), out)
			}
		}
	case "string":
		if _, ok := v.(string); !ok {
			*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: "must be a string"})
		}
	case "integer":
		n, ok := asInt(v)
		if !ok {
			*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: "must be an integer"})
			return
		}
		if sch.Minimum != nil && float64(n) < *sch.Minimum {
			*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: fmt.Sprintf("must be >= %g", *sch.Minimum)})
		}
		if sch.Maximum != nil && float64(n) > *sch.Maximum {
			*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: fmt.Sprintf("must be <= %g", *sch.Maximum)})
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			*out = append(*out, Diagnostic{Path: path, Severity: "error", Message: "must be a boolean"})
		}
	}
}

func enumMatch(v any, enum []any) bool {
	for _, e := range enum {
		if fmt.Sprint(e) == fmt.Sprint(v) {
			return true
		}
	}
	return false
}

func enumList(enum []any) string {
	parts := make([]string, len(enum))
	for i, e := range enum {
		parts[i] = fmt.Sprint(e)
	}
	return strings.Join(parts, "|")
}

func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func asSlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	default:
		return nil, false
	}
}

func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case float64:
		if math.Trunc(n) == n {
			return int64(n), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// HasErrors reports whether diagnostics contain an error.
func HasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}
