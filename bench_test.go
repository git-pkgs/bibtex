package bibtex_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-pkgs/bibtex"
)

func corpus(b *testing.B, readmes bool) []byte {
	b.Helper()
	paths, err := filepath.Glob("testdata/real-world/*")
	if err != nil {
		b.Fatal(err)
	}
	var buf bytes.Buffer
	for _, p := range paths {
		isReadme := strings.Contains(strings.ToLower(p), "readme")
		if filepath.Ext(p) == ".json" || isReadme != readmes {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			b.Fatal(err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func BenchmarkParse(b *testing.B) {
	data := corpus(b, false)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := bibtex.Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtract(b *testing.B) {
	data := corpus(b, true)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := bibtex.Extract(data, bibtex.ParseOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuthorsAndTitles(b *testing.B) {
	f, err := bibtex.Parse(corpus(b, false))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for i := range f.Entries {
			f.Entries[i].Authors()
			f.Entries[i].Text("title")
		}
	}
}
