package libauth

import (
	"testing"
)

func TestListkeys(t *testing.T) {
	keys, err := Listkeys()

	if err != nil {
		t.Error(err)
	}

	if len(keys) == 0 {
		t.Error("no keys found in factotum")
	}

	for _, k := range keys {
		t.Logf("E=%X N=%X", k.E, k.N)
	}
}
