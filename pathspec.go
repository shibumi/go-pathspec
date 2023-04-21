//
// Copyright 2014, Sander van Harmelen
// Copyright 2020, Christian Rebischke
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package pathspec

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type PathSpec struct {
	Patterns []*Pattern
}

// Append `/` to directories. Otherwise patterns that end with `/`
// won't match — a pattern that ends with a slash indicates that it
// only matches with directories.
//
// It doesn't matter whether the path
// begins with `/` or `./`; they are trimmed.
func (p *PathSpec) Match(path string) (match bool) {
	for _, pattern := range p.Patterns {
		if pattern.Match(path) {
			// match is set to true if pattern is matched and not excluded.
			// Otherwise it is set to false.
			match = !pattern.negate
		}
	}
	return
}

// MatchP matches the path and returns the matching pattern.
// Matching pattern is the last pattern that matched the path.
// Please note that the matching pattern might be the negated
// path that comes after the non-negated matching path.
//
// Append `/` to directories. Otherwise patterns that end with `/`
// won't match — a pattern that ends with a slash indicates that it
// only matches with directories.
//
// It doesn't matter whether the path
// begins with `/` or `./`; they are trimmed.
func (p *PathSpec) MatchP(path string) (pattern *Pattern, match bool) {
	for _, ptrn := range p.Patterns {
		if ptrn.Match(path) {
			// match is set to true if pattern is matched and not excluded.
			// Otherwise it is set to false.
			match = !ptrn.negate
			pattern = ptrn
		}
	}
	return
}

func FromFile(path string) (*PathSpec, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return FromReader(file)
}

func FromReader(r io.Reader) (*PathSpec, error) {
	scanner := bufio.NewScanner(r)
	const approximateLines = 20
	ptrns := make([]*Pattern, 0, approximateLines)

	for scanner.Scan() {
		line := scanner.Text()
		line = trim(line)
		if skip(line) {
			continue
		}
		p, err := parsePattern(line)
		if err != nil {
			return nil, err
		}
		ptrns = append(ptrns, p)
	}
	return &PathSpec{ptrns}, scanner.Err()
}

func FromLines(lines ...string) (*PathSpec, error) {
	ptrns := make([]*Pattern, 0, len(lines))
	for _, line := range lines {
		line = trim(line)
		if skip(line) {
			continue
		}
		p, err := parsePattern(line)
		if err != nil {
			return nil, err
		}
		ptrns = append(ptrns, p)
	}
	return &PathSpec{ptrns}, nil
}

func trim(line string) string {
	if strings.HasSuffix(line, `\ `) {
		return strings.TrimLeft(line, " ")
	}
	return strings.TrimSpace(line)
}

func skip(line string) bool {
	switch {
	case line == "":
	case len(line) > 0 && line[0] == '#':
	case line == "/":
	default:
		return false
	}
	return true
}
