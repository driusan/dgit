package git

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type PktLine string

func PktLineEncode(line []byte) (PktLine, error) {
	if len(line) > 65535 {
		return "", fmt.Errorf("Line too long to encode in PktLine format")
	}
	return PktLine(fmt.Sprintf("%.4x%s\n", len(line)+5, line)), nil
}

func PktLineEncodeNoNl(line []byte) (PktLine, error) {
	if len(line) > 65535 {
		return "", fmt.Errorf("Line too long to encode in PktLine format")
	}
	return PktLine(fmt.Sprintf("%.4x%s", len(line)+4, line)), nil
}
func (p PktLine) String() string {
	return string(p)
}

var loadLine = func(r io.Reader) string {
	size := make([]byte, 4)
	n, err := r.Read(size)
	if n != 4 || err != nil {
		return ""
	}
	val, err := strconv.ParseUint(string(size), 16, 64)
	if err != nil {
		return ""
	}
	if val == 0 {
		return ""
	}
	line := make([]byte, val-4)
	n, err = io.ReadFull(r, line)
	if uint64(n) != val-4 || err != nil {
		panic(fmt.Sprintf("Unexpected line size: %d not %d: %s", n, val, line))
	}
	return string(line)

}

func readLine(prompt string) string {
	getInput := bufio.NewReader(os.Stdin)
	var val string
	var err error
	for {
		fmt.Fprintf(os.Stderr, prompt)
		val, err = getInput.ReadString('\n')
		if err != nil {
			return ""
		}

		val = strings.TrimSpace(val)
		if val != "" {
			return val
		}
	}
}
