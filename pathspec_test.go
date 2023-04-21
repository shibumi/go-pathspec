package pathspec

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNegate(t *testing.T) {
	p, err := FromLines(
		"dead",
		"subdir/zoo",
		"foo",
		"!beef",
		"!/zoo",
		"!subdir/foo",
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, pattern := range p.Patterns[3:] {
		if !pattern.Negate() {
			t.Fatal("Negate should be true")
		}
	}

	if !p.Match("dead") {
		t.Fatal()
	}

	if p.Match("beef") {
		t.Fatal()
	}
	if p.Match("subdir/beef") {
		t.Fatal()
	}

	if p.Match("zoo") {
		t.Fatal()
	}
	if !p.Match("subdir/zoo") {
		t.Fatal()
	}

	if !p.Match("foo") {
		t.Fatal()
	}
	if p.Match("subdir/foo") {
		t.Fatal()
	}
}

func TestTrimSkipAndFromFunctions(t *testing.T) {
	lines := []string{
		// Trim
		`\ `,
		`  a\ `,
		" ",     // This will be skipped
		"     ", // This will be skipped too
		"  abc  ",
		// Skip
		"",
		"# I'm a comment",
		"/",
		"i_am_a_valid_line",
	}

	p, err := FromLines(lines...)
	if err != nil {
		t.Fatal(err)
	}
	testExpectedPatterns(t, p)

	j := strings.Join(lines, "\n")
	r := bytes.NewBufferString(j)
	p, err = FromReader(r)
	if err != nil {
		t.Fatal(err)
	}
	testExpectedPatterns(t, p)

	tempFile := filepath.Join(t.TempDir(), "pathspec_temp")
	t.Logf("tempFile: %s", tempFile)
	err = os.WriteFile(tempFile, []byte(j), 0644)
	if err != nil {
		t.Fatal(err)
	}
	p, err = FromFile(tempFile)
	if err != nil {
		t.Fatal(err)
	}
	testExpectedPatterns(t, p)
}

func testExpectedPatterns(t *testing.T, p *PathSpec) {
	expectedPatterns := []string{
		`\ `,
		`a\ `,
		"abc",
		"i_am_a_valid_line",
	}

	if len(expectedPatterns) != len(p.Patterns) {
		t.Fatalf("expected len(p.Patterns) to be %d, but it is %d", len(expectedPatterns), len(p.Patterns))
	}

	for i, pattern := range p.Patterns {
		if pattern.Pattern() != expectedPatterns[i] {
			t.Fatalf("expected %s got %s", expectedPatterns[i], pattern.Pattern())
		}
	}
}
