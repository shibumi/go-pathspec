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
	"strings"
	"testing"
)

func mustParsePattern(pattern string) *Pattern {
	p, err := parsePattern(pattern)
	if err != nil {
		panic(err)
	}
	return p
}

func TestPatterns(t *testing.T) {
	type test struct {
		name           string
		pattern        *Pattern
		shouldMatch    []string
		shouldNotMatch []string
	}
	tests := []test{
		{
			name:    "matches a plain path",
			pattern: mustParsePattern("abcdef"),
			shouldMatch: []string{
				"abcdef",
				"/abcdef",
				"subdir/abcdef",
				"/subdir/abcdef",
				"abcdef/",
				"/abcdef/",
				"subdir/abcdef/",
				"/subdir/abcdef/",
			},
			shouldNotMatch: []string{
				"someotherfile",
			},
		},
		{
			name:    "only matches directory if a backlash was appended",
			pattern: mustParsePattern("abcdef/"),
			shouldMatch: []string{
				"abcdef/",
				"/abcdef/",
				"subdir/abcdef/",
				"/subdir/abcdef/",
			},
			shouldNotMatch: []string{
				"someotherfile",
				"abcdef",
				"/abcdef",
				"subdir/abcdef",
				"/subdir/abcdef",
			},
		},
		{
			name:    "matches root path",
			pattern: mustParsePattern("/abcdef"),
			shouldMatch: []string{
				"/abcdef/",
				"/abcdef",
				"abcdef/",
				"abcdef",
			},
			shouldNotMatch: []string{
				"someotherfile",
				"subdir/abcdef",
				"/subdir/abcdef",
				"subdir/abcdef/",
				"/subdir/abcdef/",
			},
		},
		{
			name:        "should remove leading backslash",
			pattern:     mustParsePattern(`\!`),
			shouldMatch: []string{"!"},
		},
		{
			name:        "should remove leading backslash",
			pattern:     mustParsePattern(`\#`),
			shouldMatch: []string{"#"},
		},
		{
			name:    "**",
			pattern: mustParsePattern("abc/**/def"),
			shouldMatch: []string{
				"abc/daksfakgnsdjlnfdsjnfgsj/def",
				"abc/def",
				"abc/daksfakgnsdjlnfdsjnfgsj/fsdfdbfbfbfewhwbfewhbw/dsajfnewhjbfewb/def",
			},
			shouldNotMatch: []string{
				"a/cvb/def",
				"a/def",
				"abc",
				"def",
			},
		},
		{
			name:    "*",
			pattern: mustParsePattern("*.txt"),
			shouldMatch: []string{
				"file.txt",
				"CMakeLists.txt",
			},
			shouldNotMatch: []string{
				"file.gif",
				"filetxt",
			},
		},
		{
			name:    "*",
			pattern: mustParsePattern("abc*def"),
			shouldMatch: []string{
				"abcfsadfsjdgdef",
				"abcdef",
			},
			shouldNotMatch: []string{
				"abdef",
				"deadbeef",
			},
		},
		{
			name:        "*",
			pattern:     mustParsePattern("a/*"),
			shouldMatch: []string{"a/b", "a/c"},
		},
		{
			name:    "?",
			pattern: mustParsePattern("dea?beef"),
			shouldMatch: []string{
				"deadbeef",
			},
			shouldNotMatch: []string{
				"deabeef",
			},
		},

		{
			name:        "escape",
			pattern:     mustParsePattern(`file\!with\*weird\#naming_\[1\].t\?t`),
			shouldMatch: []string{`file!with*weird#naming_[1].t?t`},
		},
		{
			name:        "escape",
			pattern:     mustParsePattern(`\ \ \ `),
			shouldMatch: []string{"   "},
		},
		{
			name:           "matches only subdirectories with the `**/` pattern",
			pattern:        mustParsePattern("**/"),
			shouldMatch:    []string{"a/b", "z/e"},
			shouldNotMatch: []string{"a", "b", "c"},
		},
		{
			name:        "matches everything with the `**` pattern",
			pattern:     mustParsePattern("**"),
			shouldMatch: []string{"a", "b", "a/b", "c/d/e"},
		},
		{
			name:           "excludes characters inside square brackets negated with exclamation mark",
			pattern:        mustParsePattern("a[!bfg]c"),
			shouldMatch:    []string{"azc", "atc"},
			shouldNotMatch: []string{"abc", "afc", "agc"},
		},
		{
			name:           "excludes characters inside square brackets negated with caret",
			pattern:        mustParsePattern("a[^bfg]c"),
			shouldMatch:    []string{"azc", "atc"},
			shouldNotMatch: []string{"abc", "afc", "agc"},
		},
		{
			name:        "opening square bracket is interpreted as a character if it doesn't have a closing square bracket",
			pattern:     mustParsePattern("a[c"),
			shouldMatch: []string{"a[c"},
		},
		{
			name:        "closing square bracket is interpreted as a character if it comes directly after the opening square bracket",
			pattern:     mustParsePattern("a[]]c"),
			shouldMatch: []string{"a]c"},
		},
		{
			name:        "closing square bracket is interpreted as a character if it comes directly after the opening square bracket",
			pattern:     mustParsePattern("a[]-]bc"),
			shouldMatch: []string{"a]bc", "a-bc"},
		},
		{
			name:           "closing square bracket is interpreted as a character if it comes directly after the negation character (`!`)",
			pattern:        mustParsePattern("a[!]a-]bc"),
			shouldMatch:    []string{"agbc", "aebc"},
			shouldNotMatch: []string{"a]bc", "a-bc"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("regex: %s", test.pattern.Regex().String())

			for _, path := range test.shouldMatch {
				if !test.pattern.Match(path) {
					t.Fatalf("expected %s to match", path)
				}
			}
			for _, path := range test.shouldNotMatch {
				if test.pattern.Match(path) {
					t.Fatalf("expected %s to not match", path)
				}
			}
		})
	}
}

func TestRemoveDuplicateDoubleSlashes(t *testing.T) {
	p := mustParsePattern("/a/**/**/**/c")
	e := p.Regex().String()
	c := strings.Count(e, "(?:/.+)?")
	if c != 1 {
		t.Fatalf("expected c to be 1, but it is %d. regex: %s", c, e)
	}
}
