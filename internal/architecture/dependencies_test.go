// Package architecture guards the few dependency directions that define this
// daemon. It deliberately does not freeze filenames, sizes or every import.
package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCoreDependencyDirections(t *testing.T) {
	const module = "github.com/SilkageNet/mygardenworld/internal/"
	for _, rule := range []struct {
		pkg       string
		forbidden []string
	}{
		{"babigame", []string{"automation", "runner", "store", "apiserver", "notification", "redeem"}},
		{"state", []string{"automation", "runner", "store", "apiserver", "notification", "redeem"}},
		{"automation", []string{"runner", "store", "apiserver", "notification", "redeem"}},
		{"store", []string{"runner", "automation", "apiserver", "notification", "redeem", "state"}},
		{"notification", []string{"runner", "automation", "apiserver", "redeem"}},
	} {
		t.Run(rule.pkg, func(t *testing.T) {
			err := filepath.WalkDir(filepath.Join("..", rule.pkg), func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
				if err != nil {
					return err
				}
				for _, imp := range file.Imports {
					name, err := strconv.Unquote(imp.Path.Value)
					if err != nil {
						return err
					}
					for _, forbidden := range rule.forbidden {
						prefix := module + forbidden
						if name == prefix || strings.HasPrefix(name, prefix+"/") {
							t.Errorf("%s: %s must not depend on %s", path, rule.pkg, forbidden)
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
