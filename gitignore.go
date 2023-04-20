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

// Package pathspec implements git compatible gitignore pattern matching.
// See the description below, if you are unfamiliar with it:
//
// A blank line matches no files, so it can serve as a separator for readability.
//
// A line starting with # serves as a comment. Put a backslash ("\") in front of
// the first hash for patterns that begin with a hash.
//
// An optional prefix "!" which negates the pattern; any matching file excluded
// by a previous pattern will become included again. If a negated pattern matches,
// this will override lower precedence patterns sources. Put a backslash ("\") in
// front of the first "!" for patterns that begin with a literal "!", for example,
// "\!important!.txt".
//
// If the pattern ends with a slash, it is removed for the purpose of the following
// description, but it would only find a match with a directory. In other words,
// foo/ will match a directory foo and paths underneath it, but will not match a
// regular file or a symbolic link foo (this is consistent with the way how pathspec
// works in general in Git).
//
// If the pattern does not contain a slash /, Git treats it as a shell glob pattern
// and checks for a match against the pathname relative to the location of the
// .gitignore file (relative to the toplevel of the work tree if not from a
// .gitignore file).
//
// Otherwise, Git treats the pattern as a shell glob suitable for consumption by
// fnmatch(3) with the FNM_PATHNAME flag: wildcards in the pattern will not match
// a / in the pathname. For example, "Documentation/*.html" matches
// "Documentation/git.html" but not "Documentation/ppc/ppc.html" or/
// "tools/perf/Documentation/perf.html".
//
// A leading slash matches the beginning of the pathname. For example, "/*.c"
// matches "cat-file.c" but not "mozilla-sha1/sha1.c".
//
// Two consecutive asterisks ("**") in patterns matched against full pathname
// may have special meaning:
//
// A leading "**" followed by a slash means match in all directories. For example,
// "**/foo" matches file or directory "foo" anywhere, the same as pattern "foo".
// "**/foo/bar" matches file or directory "bar" anywhere that is directly under
// directory "foo".
//
// A trailing "/" matches everything inside. For example, "abc/" matches all files
// inside directory "abc", relative to the location of the .gitignore file, with
// infinite depth.
//
// A slash followed by two consecutive asterisks then a slash matches zero or more
// directories. For example, "a/**/b" matches "a/b", "a/x/b", "a/x/y/b" and so on.
//
// Other consecutive asterisks are considered invalid.
package pathspec

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The regex group name for the directory marker.
const DirMark = "ps_d"

type GitIgnorePattern struct {
	pattern string
	re      *regexp.Regexp
	negate  bool
}

func (p *GitIgnorePattern) Pattern() string { return p.pattern }

//nolint:staticcheck
func (p *GitIgnorePattern) Regex() *regexp.Regexp { return p.re.Copy() }

func (p *GitIgnorePattern) Negate() bool { return p.negate }

func (p *GitIgnorePattern) Match(path string) bool {
	if p.negate {
		return false
	}
	path = filepath.ToSlash(path) // Convert Windows path to Unix path
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "./")
	return p.re.MatchString(path)
}

func ParsePatternsFromFile(path string) ([]*GitIgnorePattern, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ParsePatternsFromReader(file)
}

func ParsePatternsFromReader(r io.Reader) ([]*GitIgnorePattern, error) {
	scanner := bufio.NewScanner(r)
	const approximateLines = 20
	ptrns := make([]*GitIgnorePattern, 0, approximateLines)

	for scanner.Scan() {
		pattern := scanner.Text()
		pattern = trim(pattern)
		if skip(pattern) {
			continue
		}
		p, err := parsePattern(pattern)
		if err != nil {
			return nil, err
		}
		ptrns = append(ptrns, p)
	}
	return ptrns, scanner.Err()
}

func ParsePatterns(patterns ...string) ([]*GitIgnorePattern, error) {
	ptrns := make([]*GitIgnorePattern, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = trim(pattern)
		if skip(pattern) {
			continue
		}
		p, err := parsePattern(pattern)
		if err != nil {
			return nil, err
		}
		ptrns = append(ptrns, p)
	}
	return ptrns, nil
}

func parsePattern(pattern string) (p *GitIgnorePattern, err error) {
	p = &GitIgnorePattern{
		pattern: pattern,
	}

	// An optional prefix "!" which negates the pattern; any matching file
	// excluded by a previous pattern will become included again.
	if strings.HasPrefix(pattern, "!") {
		pattern = pattern[1:]
		p.negate = true
	}

	// Remove leading back-slash escape for escaped hash ('#') or
	// exclamation mark ('!').
	pattern = strings.TrimPrefix(pattern, `\`)

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
		// Escape the character.
		switch {
		case escape:
			escape = false
			expr.WriteString(regexp.QuoteMeta(string(char)))
		case char == '\\':
			// Escape character, escape next character.
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
	if j < len(glob) && glob[j] == '!' {
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

		if glob[*i] == '!' {
			expr.WriteByte('^')
			*i++
		} else if glob[*i] == '^' {
			// POSIX declares that the regex bracket expression negation
			// "[^...]" is undefined in a glob pattern. Python's
			// `fnmatch.translate()` escapes the caret ('^') as a
			// literal. To maintain consistency with undefined behavior,
			// I am escaping the '^' as well.
			expr.WriteString(`\^`)
			*i++
		}
		expr.WriteString(regexp.QuoteMeta(glob[*i:j]))
		*i = j
	} else {
		// Failed to find closing bracket, treat opening bracket as a
		// bracket literal instead of as an expression.
		expr.WriteString(`\[`)
	}
	expr.WriteByte(']')
}

func trim(pattern string) string {
	if strings.HasSuffix(pattern, `\ `) {
		return strings.TrimLeft(pattern, " ")
	}
	return strings.TrimSpace(pattern)
}

func skip(pattern string) bool {
	switch {
	case pattern == "":
	case len(pattern) > 0 && pattern[0] == '#':
	case pattern == "/":
	default:
		return false
	}
	return true
}
