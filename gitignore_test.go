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
	"bytes"
	"strings"
	"testing"
)

func TestGitIgnore(t *testing.T) {
	toInclude := []string{"!.#test", "~foo", "foo/foo.txt", "bar/foobar.txt", "foo/bar.txt", "/bar/foo"}
	toIgnore := []string{".#test", "foo/#test#", "foo/bar/.foo.txt.swp", "foo/foobar/foobar.txt", "foo.txt", "test/foo.test", "test/foo/bar.test", "foo/bar", "foo/1/2/bar", "foo/foobar/abcd.txt", "foo/shibumi/test.txt"}
	content := []string{".#*", "\\#*#", ".*.sw[a-z]", "**/foobar/foobar.txt", "/foo.txt", "test/", "foo/**/bar", "/b[^a]r/foo", "abcd.txt", "shibumi/"}

	for _, f := range toInclude {
		match, err := GitIgnore(content, f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if match {
			t.Errorf("GitIgnore('%s', %s) returned '%v', want 'false'", content, f, match)
		}
	}

	for _, f := range toIgnore {
		match, err := GitIgnore(content, f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if !match {
			t.Errorf("GitIgnore('%s', %s) returned '%v', want 'true'", content, f, match)
		}
	}
}

func TestReadGitIgnore(t *testing.T) {
	toInclude := []string{"!.#test", "~foo", "foo/foo.txt", "bar/foobar.txt", "foo/bar.txt", "/bar/foo"}
	toIgnore := []string{".#test", "foo/#test#", "foo/bar/.foo.txt.swp", "foo/foobar/foobar.txt", "foo.txt", "test/foo.test", "test/foo/bar.test", "foo/bar", "foo/1/2/bar"}
	content := []byte(".#*\n\\#*#\n.*.sw[a-z]\n**/foobar/foobar.txt\n/foo.txt\ntest/\nfoo/**/bar\n/b[^a]r/foo")

	for _, f := range toInclude {
		match, err := ReadGitIgnore(bytes.NewReader(content), f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if match {
			t.Errorf("GitIgnore('%s', %s) returned '%v', want 'false'", strings.Replace(string(content), "\n", ", ", -1), f, match)
		}
	}

	for _, f := range toIgnore {
		match, err := ReadGitIgnore(bytes.NewReader(content), f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if !match {
			t.Errorf("GitIgnore('%s', %s) returned '%v', want 'true'", strings.Replace(string(content), "\n", ", ", -1), f, match)
		}
	}
}
