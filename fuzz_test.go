package bibtex_test

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/git-pkgs/bibtex"
)

func addSeeds(f *testing.F) {
	f.Add([]byte(software))
	f.Add([]byte("@string{a = \"b\"}\n@misc(x, title = a # {c}, author = {M{\\\"u}ller, H and others})"))
	f.Add([]byte("```bibtex\n> @misc{k,\n>  title = {T}\n```\n    @article{i, t = {x}\n"))
	paths, _ := filepath.Glob("testdata/real-world/*")
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil && filepath.Ext(p) != ".json" {
			f.Add(data)
		}
	}
}

func FuzzParse(f *testing.F) {
	addSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, extract := range []bool{false, true} {
			var file *bibtex.File
			var err error
			if extract {
				file, err = bibtex.Extract(data, bibtex.ParseOptions{})
			} else {
				file, err = bibtex.Parse(data)
			}
			if err != nil {
				continue
			}
			checkSpans(t, data, file)
			checkValues(t, data, file)
			if len(file.Diagnostics) > 101 {
				t.Fatalf("%d diagnostics", len(file.Diagnostics))
			}
		}
	})
}

// checkValues requires valid UTF-8 output from valid input and exercises
// the accessors on every entry.
func checkValues(t *testing.T, data []byte, file *bibtex.File) {
	t.Helper()
	valid := utf8.Valid(data)
	for i := range file.Entries {
		e := &file.Entries[i]
		for _, fld := range e.Fields {
			if valid && !utf8.ValidString(fld.Value) {
				t.Fatalf("invalid UTF-8 from valid input: %q", fld.Value)
			}
		}
		for _, n := range e.Authors() {
			_ = n.String()
		}
		_ = e.Text(titleField)
	}
}

func FuzzPlainText(f *testing.F) {
	for _, s := range []string{`Dvo{\v{r}}{\'a}k`, `\'{\"u}`, `$x$ -- \url{a~b}`, `{\t{oo}}`, `\href{u}{t}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := bibtex.PlainText(s)
		if utf8.ValidString(s) && !utf8.ValidString(out) {
			t.Fatalf("invalid UTF-8 output %q", out)
		}
		bibtex.ParseNames(s)
	})
}
