package bibtex_test

import (
	"crypto/sha1" //nolint:gosec // Git blob IDs are SHA-1.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/git-pkgs/bibtex"
)

type corpusEntry struct {
	Type    string   `json:"type"`
	Key     string   `json:"key"`
	Title   string   `json:"title"`
	Authors []string `json:"authors"`
	DOI     string   `json:"doi,omitempty"`
}

type corpusFixture struct {
	File        string        `json:"file"`
	Mode        string        `json:"mode"`
	Entries     []corpusEntry `json:"entries"`
	Diagnostics []string      `json:"diagnostics"`
}

// TestDownloadedCorpus checks CITATION.bib files and READMEs from GitHub
// repositories. Expected values were reviewed by hand; downloads.json
// records each source and Git blob ID.
func TestDownloadedCorpus(t *testing.T) {
	var manifest struct {
		Fixtures []corpusFixture `json:"fixtures"`
	}
	readJSON(t, "testdata/real-world/manifest.json", &manifest)
	var downloads []struct {
		File    string `json:"file"`
		GitBlob string `json:"git_blob"`
	}
	readJSON(t, "testdata/real-world/downloads.json", &downloads)
	if len(manifest.Fixtures) != len(downloads) || len(downloads) != 24 {
		t.Fatalf("manifest has %d fixtures, downloads %d", len(manifest.Fixtures), len(downloads))
	}
	blobs := map[string]string{}
	for _, d := range downloads {
		blobs[d.File] = d.GitBlob
	}
	for _, fx := range manifest.Fixtures {
		t.Run(fx.File, func(t *testing.T) {
			checkCorpusFixture(t, fx, blobs[fx.File])
		})
	}
}

func checkCorpusFixture(t *testing.T, fx corpusFixture, blob string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata/real-world", fx.File))
	if err != nil {
		t.Fatal(err)
	}
	if got := gitBlobID(data); got != blob {
		t.Fatalf("blob %s, downloads.json records %s", got, blob)
	}
	var f *bibtex.File
	if fx.Mode == "extract" {
		f, err = bibtex.Extract(data, bibtex.ParseOptions{})
	} else {
		f, err = bibtex.Parse(data)
	}
	if err != nil {
		t.Fatal(err)
	}
	checkSpans(t, data, f)
	if got := summarise(fx, f); !reflect.DeepEqual(got, fx) {
		g, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("result differs from manifest:\n%s", g)
	}
}

func summarise(fx corpusFixture, f *bibtex.File) corpusFixture {
	got := corpusFixture{File: fx.File, Mode: fx.Mode, Entries: []corpusEntry{}, Diagnostics: []string{}}
	for i := range f.Entries {
		e := &f.Entries[i]
		ce := corpusEntry{Type: e.Type, Key: e.Key, Title: e.Text(titleField), DOI: e.Get("doi"), Authors: []string{}}
		for _, a := range e.Authors() {
			ce.Authors = append(ce.Authors, a.String())
		}
		got.Entries = append(got.Entries, ce)
	}
	for _, d := range f.Diagnostics {
		got.Diagnostics = append(got.Diagnostics, d.Code)
	}
	sort.Strings(got.Diagnostics)
	return got
}

// checkSpans verifies entry offsets and positions point into data.
func checkSpans(t *testing.T, data []byte, f *bibtex.File) {
	t.Helper()
	for _, e := range f.Entries {
		if e.Start < 0 || e.End < e.Start || e.End > len(data) || data[e.Start] != '@' || e.Line < 1 || e.Column < 1 {
			t.Fatalf("entry %q at %d-%d (%d:%d) outside %d input bytes", e.Key, e.Start, e.End, e.Line, e.Column, len(data))
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

func gitBlobID(data []byte) string {
	h := sha1.New() //nolint:gosec // Git blob IDs are SHA-1.
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
