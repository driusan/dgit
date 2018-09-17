package libauth

import (
	"testing"
)

var (
	attrstr      = "proto=pass dom=example.org user=foo"
	attrexpected = map[string]string{
		"proto": "pass",
		"dom":   "example.org",
		"user":  "foo",
	}
)

func TestAttrMap(t *testing.T) {
	m := attrmap(attrstr)

	for a, v := range m {
		if ev, ok := attrexpected[a]; !ok {
			t.Errorf("attr %q not found", a)
		} else if ev != v {
			t.Errorf("got value %q expected %q", v, ev)
		}
	}
}

func TestTokenize(t *testing.T) {
	for i, tt := range []struct {
		in     string
		fields int
	}{
		{"ok a ''", 3},
		{"ok a b", 3},
		{"ok '' ''", 3},
		{"ok '' b", 3},
	} {
		spl := tokenize(tt.in)
		if len(spl) != tt.fields {
			t.Errorf("%d: expected %d fields, got %d", i, tt.fields, len(spl))
		}

		t.Logf("%d: fields: %q", i, spl)
	}
}
