package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
)

// This file provides a stupid way of parsing git config files.
// It's not very efficient, but for now it gets the job done.
// (There's a lot more low hanging fruit before optimizing this..)
type GitConfigValues map[string]string

type GitConfigSection struct {
	name, subsection string
	values           GitConfigValues
}
type GitConfig struct {
	sections []GitConfigSection
}

func (g *GitConfig) SetConfig(name, value string) {
	pieces := strings.Split(name, ".")
	var key string
	var sec *GitConfigSection
	value = strings.TrimSpace(value)
argChecker:
	switch len(pieces) {
	case 2:
		key = strings.TrimSpace(pieces[1])
		for _, section := range g.sections {
			log.Printf("Comparing %s to %s\n", section.name, pieces[0])
			if section.name == pieces[0] {
				sec = &section
				break argChecker
			}
		}
		log.Printf("Couldn't find %s, creating\n", pieces[0])
		section := GitConfigSection{pieces[0], "", make(map[string]string, 0)}
		sec = &section
		g.sections = append(g.sections, section)
	case 3:
		key = strings.TrimSpace(pieces[2])
		for _, section := range g.sections {
			log.Printf("Comparing %s to %s and %s to %s\n", section.name, pieces[0], section.subsection, pieces[1])
			if section.name == pieces[0] && section.subsection == pieces[1] {
				sec = &section
				break argChecker
			}
		}
		log.Printf("Couldn't find %s %s, creating\n", pieces[0], pieces[1])
		section := GitConfigSection{pieces[0], pieces[1], make(map[string]string, 0)}
		sec = &section
		g.sections = append(g.sections, section)
	}

	if sec != nil {
		sec.values[key] = value
	} else {
		// TODO Always auto-create the sections
		log.Printf("Couldn't find section %v\n", name)
	}
}

func (g *GitConfig) Unset(name string) int {
	pieces := strings.Split(name, ".")
	var key string
	var sec *GitConfigSection

argChecker:
	switch len(pieces) {
	case 2:
		key = strings.TrimSpace(pieces[1])
		for _, section := range g.sections {
			log.Printf("Comparing %s to %s\n", section.name, pieces[0])
			if section.name == pieces[0] {
				sec = &section
				break argChecker
			}
		}
	case 3:
		key = strings.TrimSpace(pieces[2])
		for _, section := range g.sections {
			if section.name == pieces[0] && section.subsection == pieces[1] {
				sec = &section
				break argChecker
			}
		}
	}

	if sec != nil {
		if _, ok := sec.values[key]; !ok {
			return 5
		}
		delete(sec.values, key)
		return 0
	} else {
		return 5
	}
}

func (g *GitConfig) GetConfig(name string) (string, int) {

	pieces := strings.Split(name, ".")

	switch len(pieces) {
	case 2:
		for _, section := range g.sections {
			if section.name == pieces[0] {
				val, ok := section.values[pieces[1]]
				if !ok {
					return "", 1
				}
				return val, 0
			}
		}
	case 3:
		for _, section := range g.sections {
			if section.name == pieces[0] && section.subsection == pieces[1] {
				val, ok := section.values[pieces[2]]
				if !ok {
					return "", 1
				}
				return val, 0
			}
		}

	}

	return "", 1
}

func (g *GitConfig) GetConfigList() []string {
	list := []string{}

	for _, section := range g.sections {
		for key, value := range section.values {
			if section.subsection != "" {
				list = append(list, section.name+"."+section.subsection+"."+key+"="+value)
			} else {
				list = append(list, section.name+"."+key+"="+value)
			}
		}
	}

	return list
}

func (g GitConfig) WriteFile(w io.Writer) {
	for _, section := range g.sections {
		if section.subsection == "" {
			fmt.Fprintf(w, "[%s]\n", section.name)
		} else {
			fmt.Fprintf(w, "[%s \"%s\"]\n", section.name, section.subsection)
		}

		for key, value := range section.values {
			fmt.Fprintf(w, "\t%s = %s\n", key, value)
		}

	}
}
func (s *GitConfigSection) ParseValues(valueslines string) {
	lines := strings.Split(valueslines, "\n")
	s.values = make(map[string]string)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		split := strings.Split(trimmed, "=")
		if len(split) < 1 {
			panic("couldn't parse value")
		}
		varname := strings.TrimSpace(split[0])

		log.Printf("%v\n", varname)
		s.values[varname] = strings.TrimSpace(strings.Join(split[1:], "="))

	}
}

func (s *GitConfigSection) ParseSectionHeader(headerline string) {
	s.name = headerline
	parsingSubsection := false
	subsectionStart := 0
	for idx, b := range headerline {

		if b == '"' && parsingSubsection == true {

			parsingSubsection = true
			s.subsection = strings.TrimSpace(headerline[subsectionStart:idx])

		}
		if b == '"' && parsingSubsection == false {
			parsingSubsection = true
			subsectionStart = idx + 1

			s.name = strings.TrimSpace(headerline[0:idx])

		}
	}
	if subsectionStart == 0 {
		s.name = headerline
	}
}
func ParseConfig(configFile io.Reader) GitConfig {
	rawdata, _ := ioutil.ReadAll(configFile)
	section := &GitConfigSection{}
	parsingSectionName := false
	parsingValues := false
	var sections []GitConfigSection
	lastBracket := 0
	lastClosingBracket := 0

	for idx, b := range rawdata {
		if b == '[' && parsingSectionName == false {

			parsingSectionName = true
			lastBracket = idx
			if parsingValues == true {
				section.ParseValues(string(rawdata[lastClosingBracket+1 : idx]))
				parsingValues = false
				sections = append(sections, *section)
			}
			section = &GitConfigSection{}
		}
		if b == ']' && parsingSectionName == true {
			section.ParseSectionHeader(string(rawdata[lastBracket+1 : idx]))
			parsingValues = true
			parsingSectionName = false
			lastClosingBracket = idx
		}
		if idx == len(rawdata)-1 && parsingValues == true {
			section.ParseValues(string(rawdata[lastClosingBracket+1 : idx]))
			sections = append(sections, *section)
		}
	}
	return GitConfig{sections}
}
