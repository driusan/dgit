package main

import (
	//"fmt"
	libgit "github.com/driusan/git"
	"io"
	"io/ioutil"
	"strings"
)

type GitConfigValues map[string]string

type GitConfigSection struct {
	name, subsection string
	values           GitConfigValues
}
type GitConfig struct {
	sections []GitConfigSection
}

func (g GitConfig) GetConfig(name string) string {

	pieces := strings.Split(name, ".")

	switch len(pieces) {
	case 2:
		for _, section := range g.sections {
			if section.name == pieces[0] {
				return section.values[pieces[1]]
			}
		}
	case 3:
		for _, section := range g.sections {
			if section.name == pieces[0] && section.subsection == pieces[1] {
				return section.values[pieces[2]]
			}
		}

	}
	return ""
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
func parseConfig(repo *libgit.Repository, configFile io.Reader) GitConfig {
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
		if idx == len(rawdata)-1 {
			section.ParseValues(string(rawdata[lastClosingBracket+1 : idx]))
			sections = append(sections, *section)
		}
	}
	return GitConfig{sections}
}
