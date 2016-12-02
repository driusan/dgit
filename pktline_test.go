package main

import (
	"testing"
)

func TestPktLineEncode(t *testing.T) {
	tests := []struct {
		Line    string
		Encoded PktLine
	}{
		{Line: "foo", Encoded: PktLine("0008foo\n")},
		{Line: "334a173aead888e9fb0d96eee3aa85c57cb2d8d7 3c094acaa20f8473a834cde76d044792e17c65d2\000refs/heads/AddGitPushreport-status",
			Encoded: PktLine("0079334a173aead888e9fb0d96eee3aa85c57cb2d8d7 3c094acaa20f8473a834cde76d044792e17c65d2\000refs/heads/AddGitPushreport-status\n"),
		},
	}
	for i, test := range tests {
		got, err := PktLineEncode([]byte(test.Line))
		if err != nil {
			t.Errorf("Error %v while encoding %v (TC %d)", err, test.Line, i)
		}
		if got != test.Encoded {
			t.Errorf("Error encoding %d: got %v want %v", i, got, test.Encoded)
		}
	}
}
