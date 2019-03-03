package git

import (
	"testing"
)

func TestRefSpecParsing(t *testing.T) {
	tests := []struct {
		test    RefSpec
		wantSrc Refname
		wantDst Refname
	}{
		{
			"refs/heads/foo",
			"refs/heads/foo",
			"",
		},
		{
			"+refs/heads/foo",
			"refs/heads/foo",
			"",
		},
		{
			"refs/heads/foo:refs/remotes/origin/foo",
			"refs/heads/foo",
			"refs/remotes/origin/foo",
		},
		{
			"+refs/heads/foo:refs/remotes/origin/foo",
			"refs/heads/foo",
			"refs/remotes/origin/foo",
		},
		{
			// XXX: Determine if this is a reasonable thing to do.
			"refs/heads/*:refs/remotes/origin/*",
			"refs/heads/*",
			"refs/remotes/origin/*",
		},
	}
	for i, tc := range tests {
		if got := tc.test.Src(); got != tc.wantSrc {
			t.Errorf("Test %d: unexpected Src for %v. got %v want %v", i, tc.test, got, tc.wantSrc)
		}
		if got := tc.test.Dst(); got != tc.wantDst {
			t.Errorf("Test %d: unexpected Dst for %v. got %v want %v", i, tc.test, got, tc.wantDst)
		}
	}
}
