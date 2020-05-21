package delta

import (
	"bytes"
	"container/list"
	"io/ioutil"
	"testing"
)

func TestCalculator(t *testing.T) {
	tests := []struct {
		label    string
		src, dst []byte
		want     []instruction
	}{
		{
			"No intersection",
			[]byte("abc"),
			[]byte("def"),
			[]instruction{insert("def")},
		},
		{
			"dst is prefix",
			[]byte("defabc"),
			[]byte("def"),
			[]instruction{copyinst{0, 3}},
		},
		{
			"dst is suffix",
			[]byte("abcdef"),
			[]byte("def"),
			[]instruction{copyinst{3, 3}},
		},
		{
			"src is substring of dst",
			[]byte("def"), []byte("defabc"),
			[]instruction{copyinst{0, 3}, insert("abc")},
		},
		{
			// Mostly to make sure we don't crash if < minCopy
			"small value",
			[]byte("d"), []byte("d"),
			[]instruction{insert("d")},
		},
		{
			"src is embedded in dst",
			[]byte("def"), []byte("abdefab"),
			[]instruction{
				insert("ab"),
				copyinst{0, 3},
				insert("ab"),
			},
		},
		{
			"random common substring",
			[]byte("abDxxxAxF"), []byte("AxxxFwX"),
			[]instruction{
				insert("A"),
				copyinst{3, 3},
				insert("FwX"),
			},
		},
	}

	for _, tc := range tests {
		instructions, err := calculate(tc.src, tc.dst, -1)
		if err != nil {
			t.Fatal(err)
		}
		if identicalInstructions(tc.want, instructions) != true {
			t.Errorf("%s", tc.label)
		}
	}
}

func identicalInstructions(want []instruction, got *list.List) bool {
	if len(want) != got.Len() {
		return false
	}

	i := 0
	for e := got.Front(); e != nil; e = e.Next() {
		i1 := want[i]
		if !i1.equals(e.Value.(instruction)) {
			return false
		}
		i++
	}
	return true
}

func TestCalculatorWriteInsert(t *testing.T) {
	var buf bytes.Buffer

	i := insert("abc")
	i.write(&buf)

	// simple insert instructions
	var want []byte = []byte{3, 97, 98, 99}
	if got := buf.String(); got != string(want) {
		t.Errorf("Simple insert: got %s want %s", got, want)
	}

	buf.Reset()
	// long insert, needs to generate 2 insert instructions
	// in the stream.
	// 13 sequences of 10 characters
	i = insert("0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789" +
		"0123456789")
	i.write(&buf)
	want = []byte{
		// First 127 characters
		127,
		// 0-9, x12
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57,
		// 0-6
		48, 49, 50, 51, 52, 53, 54,
		// Insert for the remaining characters
		3,
		55, 56, 57,
	}
	if got := buf.String(); got != string(want) {
		t.Errorf("Long insert: got %s want %s", got, want)
	}

}

func TestCalculatorWriteCopy(t *testing.T) {
	tests := []struct {
		label string
		i     copyinst
		want  []byte
	}{
		{
			"Length size 1 copy",
			copyinst{0, 1},
			[]byte{0x80 | 0x10, 1},
		},
		{
			"Length size 2 copy",
			copyinst{0, 1 << 8},
			[]byte{0x80 | 0x20, 1},
		},
		{
			"Length size 3 copy",
			// can't use 1 << 16 or we'd hit
			// the special case
			copyinst{0, 2 << 16},
			[]byte{0x80 | 0x40, 2},
		},
		{
			"Length size 1 and 2 copy",
			copyinst{0, (2 << 8) | 1},
			[]byte{0x80 | 0x10 | 0x20, 1, 2},
		},
		{
			"Length size 1 and 3 copy",
			copyinst{0, (3 << 16) | 1},
			[]byte{0x80 | 0x10 | 0x40, 1, 3},
		},
		{
			"Length size 2 and 3 copy",
			copyinst{0, (3 << 16) | (2 << 8)},
			[]byte{0x80 | 0x20 | 0x40, 2, 3},
		},
		{
			"Length size 1, 2 and 3 copy",
			copyinst{0, (3 << 16) | (2 << 8) | 1},
			[]byte{0x80 | 0x10 | 0x20 | 0x40, 1, 2, 3},
		},
		{
			"Length special case size copy",
			copyinst{0, 0x10000},
			[]byte{0x80},
		},
		{
			"Offset size 1 encoding",
			copyinst{1, 0x10000},
			[]byte{0x80 | 0x01, 1},
		},
		{
			"Offset size 2 encoding",
			copyinst{1 << 8, 0x10000},
			[]byte{0x80 | 0x02, 1},
		},
		{
			"Offset size 3 encoding",
			copyinst{1 << 16, 0x10000},
			[]byte{0x80 | 0x04, 1},
		},
		{
			"Offset size 4 encoding",
			copyinst{1 << 24, 0x10000},
			[]byte{0x80 | 0x08, 1},
		},
		{
			"Multibyte offset size encoding (bits 1 and 4)",
			copyinst{4<<24 | 1, 0x10000},
			[]byte{0x80 | 0x01 | 0x08, 1, 4},
		},
		{
			"Mixed offset and length encoding",
			copyinst{1<<8 | 2, 3},
			[]byte{
				0x80 |
					0x1 | 0x2 | // offset bits
					0x10, // length bits
				2, 1, // offset first
				3, // length second
			},
		},
	}
	var buf bytes.Buffer

	for _, tc := range tests {
		buf.Reset()
		tc.i.write(&buf)

		if got := buf.String(); got != string(tc.want) {
			t.Errorf("%s: got %v want %v", tc.label, []byte(got), tc.want)
		}
	}
}

// Calculate a delta and ensure reading it resolves to the same
// value
func TestSanityTest(t *testing.T) {
	// A random 2 instruction from TestCalculator.
	// src is "def", dst is "defabc". Should result
	// in both a copy and an insert.
	// (was tested in TestCalculator)
	var delta bytes.Buffer
	base := []byte("def")
	target := []byte("defabc")
	Calculate(&delta, base, target, -1)

	resolved := NewReader(
		bytes.NewReader(delta.Bytes()),
		bytes.NewReader(base),
	)
	val, err := ioutil.ReadAll(&resolved)
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != string(target) {
		t.Errorf("Unexpected delta resolution: got %v want %v", val, target)
	}

	println("Doing delta that we want")
	// "Large" delta, one which gave us problems with the git test suite..
	base = make([]byte, 4096)

	for i := range base {
		base[i] = 'c'
	}
	target = []byte(string(base) + "foo")
	delta.Reset()
	Calculate(&delta, base, target, -1)

	resolved = NewReader(
		bytes.NewReader(delta.Bytes()),
		bytes.NewReader(base),
	)
	val, err = ioutil.ReadAll(&resolved)
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != string(target) {
		t.Errorf("Unexpected delta resolution: got %v want %v", val, target)
	}
}
