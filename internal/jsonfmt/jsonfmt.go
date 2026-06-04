// Package jsonfmt renders JSON byte-for-byte the way becket's bash/python writer
// does: no HTML escaping, a trailing newline, and a fixed indent. Indent2 mirrors
// `json.dump(..., indent=2)` (config + manifests); Indent4 mirrors
// `python -m json.tool` (the --output json views). It also recovers object key
// order from raw JSON so Go maps can be re-marshaled in document order.
package jsonfmt

import (
	"bytes"
	"encoding/json"
)

func encode(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil { // Encode appends the trailing newline
		return nil, err
	}
	return buf.Bytes(), nil
}

// Indent2 renders with two-space indent (settings.json, .becket.json).
func Indent2(v any) ([]byte, error) { return encode(v, "  ") }

// Indent4 renders with four-space indent (python -m json.tool parity).
func Indent4(v any) ([]byte, error) { return encode(v, "    ") }

// NestedKeyOrder returns the keys, in document order, of the object under the
// given top-level field of raw JSON (empty if absent or not an object).
func NestedKeyOrder(raw []byte, field string) []string {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return nil
	}
	obj, ok := top[field]
	if !ok {
		return nil
	}
	return objectKeyOrder(obj)
}

func objectKeyOrder(obj json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(obj))
	if t, err := dec.Token(); err != nil {
		return nil
	} else if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil
	}
	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return keys
		}
		key, _ := t.(string)
		keys = append(keys, key)
		if err := skipValue(dec); err != nil {
			return keys
		}
	}
	return keys
}

func skipValue(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); ok && (d == '{' || d == '[') {
		depth := 1
		for depth > 0 {
			t, err := dec.Token()
			if err != nil {
				return err
			}
			if d, ok := t.(json.Delim); ok {
				if d == '{' || d == '[' {
					depth++
				} else {
					depth--
				}
			}
		}
	}
	return nil
}
