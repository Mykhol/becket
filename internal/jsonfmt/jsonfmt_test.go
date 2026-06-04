package jsonfmt

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// orderedStruct exercises struct field-order preservation: Go marshals struct
// fields in declaration order, regardless of name, so Zebra must precede Apple.
type orderedStruct struct {
	Zebra  string `json:"zebra"`
	Apple  int    `json:"apple"`
	Middle bool   `json:"middle"`
}

func TestIndent2(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "html chars are not escaped",
			in:   map[string]string{"k": "a<b>c&d"},
			want: "{\n  \"k\": \"a<b>c&d\"\n}\n",
		},
		{
			name: "map keys sorted",
			in:   map[string]int{"c": 3, "a": 1, "b": 2},
			want: "{\n  \"a\": 1,\n  \"b\": 2,\n  \"c\": 3\n}\n",
		},
		{
			name: "struct field order preserved",
			in:   orderedStruct{Zebra: "z", Apple: 1, Middle: true},
			want: "{\n  \"zebra\": \"z\",\n  \"apple\": 1,\n  \"middle\": true\n}\n",
		},
		{
			name: "nested object uses two-space indent per level",
			in:   map[string]any{"outer": map[string]any{"inner": 1}},
			want: "{\n  \"outer\": {\n    \"inner\": 1\n  }\n}\n",
		},
		{
			name: "array elements",
			in:   []int{1, 2, 3},
			want: "[\n  1,\n  2,\n  3\n]\n",
		},
		{
			name: "empty object",
			in:   map[string]any{},
			want: "{}\n",
		},
		{
			name: "scalar string",
			in:   "hi",
			want: "\"hi\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Indent2(tt.in)
			if err != nil {
				t.Fatalf("Indent2 returned error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Indent2 bytes mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestIndent4(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "html chars are not escaped",
			in:   map[string]string{"k": "a<b>c&d"},
			want: "{\n    \"k\": \"a<b>c&d\"\n}\n",
		},
		{
			name: "map keys sorted",
			in:   map[string]int{"c": 3, "a": 1, "b": 2},
			want: "{\n    \"a\": 1,\n    \"b\": 2,\n    \"c\": 3\n}\n",
		},
		{
			name: "struct field order preserved",
			in:   orderedStruct{Zebra: "z", Apple: 1, Middle: true},
			want: "{\n    \"zebra\": \"z\",\n    \"apple\": 1,\n    \"middle\": true\n}\n",
		},
		{
			name: "nested object uses four-space indent per level",
			in:   map[string]any{"outer": map[string]any{"inner": 1}},
			want: "{\n    \"outer\": {\n        \"inner\": 1\n    }\n}\n",
		},
		{
			name: "array elements",
			in:   []int{1, 2, 3},
			want: "[\n    1,\n    2,\n    3\n]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Indent4(tt.in)
			if err != nil {
				t.Fatalf("Indent4 returned error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Indent4 bytes mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// TestIndentTrailingNewline asserts there is exactly one trailing newline and
// no other framing characters around the encoded value.
func TestIndentTrailingNewline(t *testing.T) {
	fns := map[string]func(any) ([]byte, error){
		"Indent2": Indent2,
		"Indent4": Indent4,
	}
	for name, fn := range fns {
		t.Run(name, func(t *testing.T) {
			got, err := fn(map[string]int{"a": 1})
			if err != nil {
				t.Fatalf("%s returned error: %v", name, err)
			}
			if !bytes.HasSuffix(got, []byte("}\n")) {
				t.Errorf("%s output does not end with \"}\\n\": %q", name, got)
			}
			if bytes.HasSuffix(got, []byte("\n\n")) {
				t.Errorf("%s output has more than one trailing newline: %q", name, got)
			}
			if n := bytes.Count(got, []byte("\n")); n != 3 {
				// {\n  "a": 1\n}\n  -> exactly 3 newlines for a single-key object.
				t.Errorf("%s expected 3 newlines, got %d: %q", name, n, got)
			}
		})
	}
}

// TestIndentError confirms encode propagates marshaling errors instead of
// returning bytes.
func TestIndentError(t *testing.T) {
	for _, fn := range []func(any) ([]byte, error){Indent2, Indent4} {
		got, err := fn(make(chan int)) // channels are not JSON-marshalable
		if err == nil {
			t.Errorf("expected error for unmarshalable value, got bytes: %q", got)
		}
		if got != nil {
			t.Errorf("expected nil bytes on error, got: %q", got)
		}
	}
}

func TestNestedKeyOrder(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		field string
		want  []string
	}{
		{
			name:  "keys returned in document order not alphabetical",
			raw:   `{"obj": {"zebra": 1, "apple": 2, "middle": 3}}`,
			field: "obj",
			want:  []string{"zebra", "apple", "middle"},
		},
		{
			name:  "nested objects and arrays are skipped, only top-level keys returned",
			raw:   `{"obj": {"a": {"deep": 1}, "b": [1, 2, {"x": 3}], "c": "v"}}`,
			field: "obj",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "deeply nested and mixed values still yield only direct keys in order",
			raw:   `{"obj": {"first": [[1], {"n": [2]}], "second": {"a": {"b": {"c": 1}}}, "third": null}}`,
			field: "obj",
			want:  []string{"first", "second", "third"},
		},
		{
			name:  "missing field returns nil",
			raw:   `{"other": {"a": 1}}`,
			field: "obj",
			want:  nil,
		},
		{
			name:  "field present but value is an array returns nil",
			raw:   `{"obj": [1, 2, 3]}`,
			field: "obj",
			want:  nil,
		},
		{
			name:  "field present but value is a scalar returns nil",
			raw:   `{"obj": 42}`,
			field: "obj",
			want:  nil,
		},
		{
			name:  "empty object returns empty (nil) slice",
			raw:   `{"obj": {}}`,
			field: "obj",
			want:  nil,
		},
		{
			name:  "invalid json returns nil",
			raw:   `{not valid json`,
			field: "obj",
			want:  nil,
		},
		{
			name:  "top-level is not an object returns nil",
			raw:   `[1, 2, 3]`,
			field: "obj",
			want:  nil,
		},
		{
			name:  "duplicate keys are both reported in order",
			raw:   `{"obj": {"a": 1, "b": 2, "a": 3}}`,
			field: "obj",
			want:  []string{"a", "b", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NestedKeyOrder([]byte(tt.raw), tt.field)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NestedKeyOrder(%q, %q) = %#v, want %#v", tt.raw, tt.field, got, tt.want)
			}
		})
	}
}

// TestNestedKeyOrderRoundTrip is a property-style check: the keys reported for a
// field should match the order json.Decoder yields them when decoding that
// field's object directly.
func TestNestedKeyOrderRoundTrip(t *testing.T) {
	raw := []byte(`{"meta": {"name": "becket", "version": "1.0", "deps": ["a", "b"], "nested": {"x": 1}}}`)
	got := NestedKeyOrder(raw, "meta")
	want := []string{"name", "version", "deps", "nested"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NestedKeyOrder = %#v, want %#v", got, want)
	}

	// Sanity: ensure the field really is a valid object (guards against the test
	// fixture rotting).
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("fixture is not valid JSON: %v", err)
	}
	if _, ok := top["meta"]; !ok {
		t.Fatalf("fixture missing meta field")
	}
}
