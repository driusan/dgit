package cmd

import (
	"flag"
	"testing"
)

func TestAliasedStringValue(t *testing.T) {
	flags := flag.NewFlagSet("test1", flag.ContinueOnError)
	v := "" // Start as zero value
	flags.Var(newAliasedStringValue(&v, ""), "foo", "")
	flags.Var(newAliasedStringValue(&v, ""), "bar", "")

	err := flags.Parse([]string{"--foo=one"})
	if err != nil {
		panic(err)
	}
	if v != "one" {
		t.Fail()
	}

	err = flags.Parse([]string{"--foo=one", "--bar=two"})
	if err == nil {
		t.Fail()
	}

	err = flags.Parse([]string{"--foo=one", "--foo=two"})
	if err == nil {
		t.Fail()
	}
}

func TestMultiStringValue(t *testing.T) {
	flags := flag.NewFlagSet("test2", flag.ContinueOnError)
	var v []string // Start as a zero value
	flags.Var(newMultiStringValue(&v), "foo", "")

	err := flags.Parse([]string{"--foo=one", "--foo=two"})
	if err != nil {
		panic(err)
	}

	if len(v) != 2 {
		t.Fail()
	}
	if v[0] != "one" || v[1] != "two" {
		t.Fail()
	}
}
