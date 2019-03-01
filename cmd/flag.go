package cmd

import (
	"fmt"
)

// A string value compatible with a flag var
//  that allows you to assign multiple flags
//  to the same string value. If the value is
//  set twice either by duplicating the flag
//  or using the original and alias then an error
//  is raised.
type aliasedStringValue string

func newAliasedStringValue(p *string, val string) *aliasedStringValue {
	*p = val
	return (*aliasedStringValue)(p)
}

func (s *aliasedStringValue) Set(val string) error {
	if *s != "" {
		return fmt.Errorf("Value already set to %v", val)
	}
	*s = aliasedStringValue(val)
	return nil
}

func (s *aliasedStringValue) Get() interface{} { return string(*s) }

func (s *aliasedStringValue) String() string { return string(*s) }

// A string value compatible with a flag var
//  that allows you to assign multiple strings
//  to the same string value as a string slice.
type multiStringValue []string

func NewMultiStringValue(p *[]string) *multiStringValue {
	return (*multiStringValue)(p)
}

func (s *multiStringValue) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func (s *multiStringValue) Get() interface{} { return []string(*s) }

func (s *multiStringValue) String() string { return fmt.Sprintf("%v\n", *s) }

// A string value that indicates that it is not yet implemented if it's used.
type notimplStringValue string

func newNotimplStringValue() *notimplStringValue {
	var s string
	return (*notimplStringValue)(&s)
}

func (s *notimplStringValue) Set(val string) error {
	return fmt.Errorf("Not yet implemented")
}

func (s *notimplStringValue) Get() interface{} { return string(*s) }

func (s *notimplStringValue) String() string { return string(*s) }

// A boolean value that indicates that it is not yet implemented if it's used.
type notimplBoolValue bool

func newNotimplBoolValue() *notimplBoolValue {
	var b bool
	return (*notimplBoolValue)(&b)
}

func (b *notimplBoolValue) Set(val string) error {
	return fmt.Errorf("Not yet implemented")
}

func (b *notimplBoolValue) Get() interface{} { return bool(*b) }

func (b *notimplBoolValue) String() string { return "false" }

func (b *notimplBoolValue) IsBoolFlag() bool { return true }
