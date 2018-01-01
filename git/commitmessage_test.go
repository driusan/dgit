package git

import (
	"testing"
)

// Tests that the various cleanup modes for git commit messages
// work as expected.
func TestCommitMessageCleanup(t *testing.T) {
	tests := []struct {
		CommitMessage CommitMessage
		Whitespace    string
		Strip         string
	}{
		{
			"I am a test",
			"I am a test\n",
			"I am a test\n",
		},
		{
			"\n\nI am a test\n\n\n\n",
			"I am a test\n",
			"I am a test\n",
		},
		{
			"I am a paragraph\n\n\nI had an extra newline",
			"I am a paragraph\n\nI had an extra newline\n",
			"I am a paragraph\n\nI had an extra newline\n",
		},
		{
			`I am here

I am another paragraph
# pretend I am a status message`,
			`I am here

I am another paragraph
# pretend I am a status message
`,
			`I am here

I am another paragraph
`,
		},
	}
	for i, tc := range tests {
		if got := tc.CommitMessage.whitespace(); got != tc.Whitespace {
			t.Errorf("Case %d whitespace: got %v want %v", i, got, tc.Whitespace)
		}
		if got := tc.CommitMessage.strip(); got != tc.Strip {
			t.Errorf("Case %d strip: got %v want %v", i, got, tc.Strip)
		}
	}
}
