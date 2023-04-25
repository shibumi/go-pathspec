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
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// The regex group name for the directory marker.
const DirMark = "ps_d"

type Pattern struct {
	pattern string
	re      *regexp.Regexp
	negate  bool
}

func (p *Pattern) Pattern() string { return p.pattern }

//nolint:staticcheck
func (p *Pattern) Regex() *regexp.Regexp { return p.re.Copy() }

func (p *Pattern) Negate() bool { return p.negate }

func (p *Pattern) Match(path string) bool {
	path = filepath.ToSlash(path) // Convert Windows path to Unix path
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "./")
	return p.re.MatchString(path)
}

func parsePattern(pattern string) (p *Pattern, err error) {
	p = &Pattern{
		pattern: pattern,
	}

	// An optional prefix "!" which negates the pattern; any matching file
	// excluded by a previous pattern will become included again.
	if strings.HasPrefix(pattern, "!") {
		pattern = pattern[1:]
		p.negate = true
	}

	// Split pattern into segments.
	patternSegs := strings.Split(pattern, "/")

	// EDGE CASE: Deal with duplicate double-asterisk sequences.
	// Collapse each sequence down to one double-asterisk. Iterate over
	// the segments in reverse and remove the duplicate double
	// asterisks as we go.
	for i := len(patternSegs) - 1; i > 0; i-- {
		prev := patternSegs[i-1]
		seg := patternSegs[i]
		if prev == "**" && seg == "**" {
			// Remove
			patternSegs = append(patternSegs[:i], patternSegs[i+1:]...)
		}
	}

	// EDGE CASE: The '**/' pattern should match everything except
	// individual files in the root directory. This case cannot be
	// adequately handled through normalization. Use the override.
	if len(patternSegs) == 2 && patternSegs[0] == "**" && patternSegs[1] == "" {
		p.re = regexp.MustCompile(fmt.Sprintf("^.+(?P<%s>/).*$", DirMark))
		return p, nil
	}

	if patternSegs[0] == "" {
		// A pattern beginning with a slash ('/') will only match paths
		// directly on the root directory instead of any descendant paths.
		// So remove empty first segment to make pattern absoluut to root.
		// A pattern without a beginning slash ('/') will match any
		// descendant path. This is equivilent to "**/{pattern}". So
		// prepend with double-asterisks to make pattern relative to
		// root.
		patternSegs = patternSegs[1:]
	} else if len(patternSegs) == 1 || (len(patternSegs) == 2 && patternSegs[1] == "") {
		// A single pattern without a beginning slash ('/') will match
		// any descendant path. This is equivalent to "**/{pattern}". So,
		// prepend with double-asterisks to make pattern relative to
		// root.
		// EDGE CASE: This also holds for a single pattern with a
		// trailing slash (e.g. dir/).
		if patternSegs[0] != "**" {
			patternSegs = append([]string{"**"}, patternSegs...)
		}
	} else {
		// EDGE CASE: A pattern without a beginning slash ('/') but
		// contains at least one prepended directory (e.g.
		// "dir/{pattern}") should not match "**/dir/{pattern}",
		// according to `git check-ignore` (v2.4.1).
	}

	// A pattern ending with a slash ('/') will match all descendant
	// paths of if it is a directory but not if it is a regular file.
	// This is equivalent to "{pattern}/**". So, set last segment to
	// double asterisks to include all descendants.
	if patternSegs[len(patternSegs)-1] == "" && len(patternSegs) > 1 {
		patternSegs[len(patternSegs)-1] = "**"
	}

	// Build regular expression from pattern.
	expr := strings.Builder{}
	needSlash := false
	end := len(patternSegs) - 1

	const approximateExtraLen = 30
	expr.Grow(len(pattern) + approximateExtraLen)
	expr.WriteString("^")

	for i, seg := range patternSegs {
		switch seg {
		case "**":
			switch {
			case i == 0 && i == end:
				// A pattern consisting solely of double-asterisks ('**')
				// will match every path.
				expr.WriteString(fmt.Sprintf("[^/]+(?:(?P<%s>/).*)?", DirMark))
			case i == 0:
				// A normalized pattern beginning with double-asterisks
				// ('**') will match any leading path segments.
				expr.WriteString("(?:.+/)?")
				needSlash = false
			case i == end:
				// A normalized pattern ending with double-asterisks ('**')
				// will match any trailing path segments.
				expr.WriteString(fmt.Sprintf("(?P<%s>/).*", DirMark))
			default:
				// A pattern with inner double-asterisks ('**') will match
				// multiple (or zero) inner path segments.
				expr.WriteString("(?:/.+)?")
				needSlash = true
			}
		case "*":
			// Match single path segment.
			if needSlash {
				expr.WriteString("/")
			}
			expr.WriteString("[^/]+")
			if i == end {
				// A pattern ending without a slash ('/') will match a file
				// or a directory (with paths underneath it). E.g., "foo"
				// matches "foo", "foo/bar", "foo/bar/baz", etc.
				expr.WriteString(fmt.Sprintf("(?:(?P<%s>/).*)?", DirMark))
			}
			needSlash = true
		default:
			// Match segment glob pattern.
			if needSlash {
				expr.WriteString("/")
			}
			translateGlob(&expr, seg)
			if i == end {
				// A pattern ending without a slash ('/') will match a file
				// or a directory (with paths underneath it). E.g., "foo"
				// matches "foo", "foo/bar", "foo/bar/baz", etc.
				expr.WriteString(fmt.Sprintf("(?:(?P<%s>/).*)?", DirMark))
			}
			needSlash = true
		}
	}

	expr.WriteString("$")
	p.re, err = regexp.Compile(expr.String())
	return p, err
}

// NOTE: This is derived from `fnmatch.translate()` and is similar to
// the POSIX function `fnmatch()` with the `FNM_PATHNAME` flag set.
func translateGlob(expr *strings.Builder, glob string) {
	escape := false

	for i := 0; i < len(glob); i++ {
		char := glob[i]

		switch {
		case escape:
			escape = false
			expr.WriteString(regexp.QuoteMeta(string(char)))
		case char == '\\':
			// Escape the next character.
			escape = true
		case char == '*':
			// Multi-character wildcard. Match any string (except slashes),
			// including an empty string.
			expr.WriteString("[^/]*")
		case char == '?':
			// Single-character wildcard. Match any single character (except
			// a slash).
			expr.WriteString("[^/]")
		case char == '[':
			translateBracketExpression(expr, &i, glob)
		default:
			// Regular character, escape it for regex.
			expr.WriteString(regexp.QuoteMeta(string(char)))
		}
	}
}

// Bracket expression wildcard. Except for the beginning
// exclamation mark, the whole bracket expression can be used
// directly as regex but we have to find where the expression
// ends.
// - "[][!]" matches ']', '[' and '!'.
// - "[]-]" matches ']' and '-'.
// - "[!]a-]" matches any character except ']', 'a' and '-'.
func translateBracketExpression(expr *strings.Builder, i *int, glob string) {
	// Move past the opening bracket.
	*i++
	// From now on, use j as the index.
	j := *i

	// Pass bracket expression negation.
	if j < len(glob) && (glob[j] == '!' || glob[j] == '^') {
		j++
	}
	// Pass first closing bracket if it is at the beginning of the
	// expression.
	if j < len(glob) && glob[j] == ']' {
		j++
	}
	// Find closing bracket. Stop once we reach the end or find it.
	for j < len(glob) && glob[j] != ']' {
		j++
	}

	if j < len(glob) {
		// Found end of bracket expression. Increment j to be one past
		// the closing bracket:
		//
		//  [...]
		//   ^   ^
		//   i   j
		//
		j++
		expr.WriteByte('[')

		if glob[*i] == '!' || glob[*i] == '^' {
			expr.WriteByte('^')
			*i++
		}

		expr.WriteString(strings.ReplaceAll(glob[*i:j], `\`, `\\`))
		// Subtract 1 because i will be incremented at the end
		// of the for loop of translateGlob
		*i = j - 1
	} else {
		// Failed to find closing bracket, treat opening bracket as a
		// bracket literal instead of as an expression.
		expr.WriteString(`\[`)
		// Subtract 1 because i will be incremented at the end
		// of the for loop of translateGlob
		*i--
	}
}
