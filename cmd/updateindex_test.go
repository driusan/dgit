package cmd

import (
	"testing"

	"github.com/driusan/dgit/git"
)

func sha1FromString(s string) git.Sha1 {
	sha1, err := git.Sha1FromString(s)
	if err != nil {
		panic(err)
	}
	return sha1
}
func TestUpdateIndexCacheInfoCmd(t *testing.T) {
	tests := []struct {
		input       string
		expected    git.CacheInfo
		expectedErr bool
	}{
		{
			"103412,fbca232",
			git.CacheInfo{},
			true,
		},
		{
			"100755,c69ecb78f2115e92c1baa9887226d158fe5bfeda,foo",
			git.CacheInfo{
				git.ModeExec,
				sha1FromString("c69ecb78f2115e92c1baa9887226d158fe5bfeda"),
				"foo",
			},
			false,
		},
	}
	for i, tc := range tests {
		ci, err := parseCacheInfo(tc.input)
		if err == nil && tc.expectedErr {
			t.Errorf("Case %d: Expected error, got none", i)
		} else if err != nil && !tc.expectedErr {
			t.Errorf("Case %d: Did not expect error, got %v", i, err)
		}

		if ci != tc.expected {
			t.Errorf("Case %d: got %v want %v", i, ci, tc.expected)
		}

	}
}
