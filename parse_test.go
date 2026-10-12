package bibtex_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/bibtex"
)

const (
	titleField = "title"
	monthField = "month"
	doe        = "Doe"
	jane       = "Jane"
	vanDer     = "van der"
	deLa       = "de la"
	jrDot      = "Jr."
	muller     = "Müller"
)

const software = `% exported from Zenodo
@software{doe_2024_example,
  author    = {Doe, Jane and van der Berg, Jan and {Example Consortium}},
  title     = {Example: A {GPU} library},
  year      = 2024,
  month     = jun,
  version   = {v1.2.3},
  doi       = {10.5281/zenodo.123},
  url       = {https://example.org/a_b%20c#frag}
}
`

func TestParseSoftwareEntry(t *testing.T) {
	f, err := bibtex.Parse([]byte(software))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 1 || len(f.Diagnostics) != 0 || f.Encoding != "utf-8" {
		t.Fatalf("unexpected result: %+v", f)
	}
	e := &f.Entries[0]
	if e.Type != "software" || e.Key != "doe_2024_example" || e.Line != 2 || e.Column != 1 {
		t.Fatalf("entry header: %+v", e)
	}
	want := map[string]string{
		titleField: "Example: A {GPU} library",
		"year":     "2024",
		monthField: "June",
		"version":  "v1.2.3",
		"doi":      "10.5281/zenodo.123",
		"url":      "https://example.org/a_b%20c#frag",
	}
	for k, v := range want {
		if got := e.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if e.Text(titleField) != "Example: A GPU library" {
		t.Errorf("plain title %q", e.Text(titleField))
	}
	if f := e.Fields[1]; f.Name != titleField || f.Line != 4 || f.Column != 3 {
		t.Errorf("field position: %+v", f)
	}
	authors := e.Authors()
	wantAuthors := []bibtex.Name{
		{First: jane, Last: doe},
		{First: "Jan", Von: vanDer, Last: "Berg"},
		{Last: "Example Consortium"},
	}
	if !reflect.DeepEqual(authors, wantAuthors) {
		t.Errorf("authors %+v", authors)
	}
	if at := strings.IndexByte(software, '@'); at < 0 || software[e.Start:e.End] != strings.TrimSpace(software[at:]) {
		t.Errorf("span %q", software[e.Start:e.End])
	}
}

func TestParseSyntax(t *testing.T) {
	tests := []struct {
		name, src         string
		key, field, value string
		diags             []string
	}{
		{"parentheses", `@misc(k, title = {T})`, "k", titleField, "T", nil},
		{"quoted", `@misc{k, title = "A {"}quoted{"} value"}`, "k", titleField, `A {"}quoted{"} value`, nil},
		{"concatenation", "@string{pre = \"Proc. \"}\n@misc{k, booktitle = pre # {ICSE} # \" 2024\"}", "k", "booktitle", "Proc. ICSE 2024", nil},
		{"string parentheses", "@string(x = {X})\n@misc{k, note = x}", "k", "note", "X", nil},
		{"macro case", "@STRING{Jnl = {J}}\n@misc{k, journal = JNL}", "k", "journal", "J", nil},
		{monthField, `@misc{k, month = OCT}`, "k", monthField, "October", nil},
		{"undefined macro", `@misc{k, booktitle = CCS}`, "k", "booktitle", "CCS", []string{"undefined_macro"}},
		{"number", `@misc{k, year = 2021}`, "k", "year", "2021", nil},
		{"whitespace", "@misc{k, title = {  A\n\t  long\r\n title  }}", "k", titleField, "A long title", nil},
		{"uppercase", `@ARTICLE{Key, TITLE = {T}}`, "Key", titleField, "T", nil},
		{"space after at", `@ misc { k , title = {T} , }`, "k", titleField, "T", nil},
		{"trailing comma", "@misc{k,\n title = {T},\n}", "k", titleField, "T", nil},
		{"crlf", "@misc{k,\r\n title = {T}\r\n}\r\n", "k", titleField, "T", nil},
		{"bom", "\ufeff@misc{k, title = {T}}", "k", titleField, "T", nil},
		{"non-breaking space", "@misc{k,\n  title = {T}}", "k", titleField, "T", nil},
		{"key with punctuation", `@misc{Zhang:2007:WW/1.2-3, title = {T}}`, "Zhang:2007:WW/1.2-3", titleField, "T", nil},
		{"missing comma after key", "@book{k\n title = {T}}", "k", titleField, "T", []string{"missing_comma"}},
		{"missing comma between fields", "@misc{k, note = {N}\n title = {T}}", "k", titleField, "T", []string{"missing_comma"}},
		{"missing key", `@misc{title = {T}}`, "", titleField, "T", []string{"missing_key"}},
		{"empty key", `@misc{, title = {T}}`, "", titleField, "T", []string{"missing_key"}},
		{"duplicate field", `@misc{k, title = {T}, Title = {U}}`, "k", titleField, "T", []string{"duplicate_field"}},
		{"missing equals", `@misc{k, note {N}, title = {T}}`, "k", titleField, "T", []string{"missing_equals"}},
		{"junk before field", `@misc{k, ; title = {T}}`, "k", titleField, "T", []string{"missing_equals"}},
		{"unexpected", `@misc{k, ) title = {T}}`, "k", titleField, "T", []string{"unexpected_character"}},
		{"unterminated entry", "@misc{k, title = {T}", "k", titleField, "T", []string{"unterminated_entry"}},
		{"stray closing brace", `@misc{k, title = "A } B"}`, "k", titleField, "A } B", []string{"unbalanced_braces"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := bibtex.Parse([]byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Entries) != 1 {
				t.Fatalf("entries: %+v diagnostics: %+v", f.Entries, f.Diagnostics)
			}
			e := f.Entries[0]
			if e.Key != tc.key || e.Get(tc.field) != tc.value {
				t.Fatalf("key %q, %s = %q", e.Key, tc.field, e.Get(tc.field))
			}
			if got := codes(f.Diagnostics); !reflect.DeepEqual(got, tc.diags) {
				t.Fatalf("diagnostics %v, want %v", got, tc.diags)
			}
		})
	}
}

func TestParseCommentsAndJunk(t *testing.T) {
	src := `Text before entries, with an email user@example.org in it.
@comment{jabref-meta: databaseType:bibtex;}
@Comment This line is ignored
@preamble{"\newcommand{\noop}[1]{}"}
@misc{a, title = {A}}
@misc{b, title = {B}}

stray text between entries
@misc{a, title = {Duplicate}}
`
	f, err := bibtex.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if keys := entryKeys(f); !reflect.DeepEqual(keys, []string{"a", "b", "a"}) {
		t.Fatalf("keys %v", keys)
	}
	if len(f.Preambles) != 1 || f.Preambles[0] != `\newcommand{\noop}[1]{}` {
		t.Fatalf("preambles %q", f.Preambles)
	}
	if got := codes(f.Diagnostics); !reflect.DeepEqual(got, []string{"duplicate_key"}) {
		t.Fatalf("diagnostics %v", got)
	}
}

func TestParseRecoversAfterUnterminatedValue(t *testing.T) {
	for _, src := range []string{
		"@misc{a, title = {Broken\n}\n@misc{b, title = {B}}\n",
		"@misc{a, title = \"Broken\n@misc{b, title = {B}}\n",
		"@misc{a, title = \"Broken",
	} {
		f, err := bibtex.Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if len(f.Diagnostics) == 0 || f.Entries[0].Key != "a" {
			t.Fatalf("%q: %+v", src, f)
		}
		if strings.Contains(src, "@misc{b") && (len(f.Entries) != 2 || f.Entries[1].Get(titleField) != "B") {
			t.Fatalf("%q: following entry lost: %+v", src, f.Entries)
		}
	}
}

func TestParseWindows1252(t *testing.T) {
	src := []byte("@misc{k, author = {M\xfcller, J\xf6rg}, title = {\x93Quoted\x94}}")
	f, err := bibtex.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	e := f.Entries[0]
	if f.Encoding != "windows-1252" || e.Get(titleField) != "“Quoted”" || e.Authors()[0] != (bibtex.Name{First: "Jörg", Last: muller}) {
		t.Fatalf("decoded %q %+v", e.Get(titleField), e.Authors())
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		opts bibtex.ParseOptions
		want error
	}{
		{"utf-16", []byte{0xFF, 0xFE, '@', 0}, bibtex.ParseOptions{}, bibtex.ErrUnsupported},
		{"negative", nil, bibtex.ParseOptions{MaxEntries: -1}, bibtex.ErrOptions},
		{"bytes", []byte("@misc{k, title = {T}}"), bibtex.ParseOptions{MaxBytes: 5}, bibtex.ErrLimit},
		{"entries", []byte("@misc{a,}\n@misc{b,}"), bibtex.ParseOptions{MaxEntries: 1}, bibtex.ErrLimit},
		{"value", []byte("@misc{k, title = {0123456789}}"), bibtex.ParseOptions{MaxValueBytes: 4}, bibtex.ErrLimit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := bibtex.ParseWithOptions(tc.data, tc.opts)
			if !errors.Is(err, tc.want) || f != nil {
				t.Fatalf("got %v, %v", f, err)
			}
			var e *bibtex.Error
			if !errors.As(err, &e) || e.Code == "" {
				t.Fatalf("error lacks diagnostic: %v", err)
			}
		})
	}
}

func TestDiagnosticLimit(t *testing.T) {
	src := strings.Repeat("@misc{k, x = undefined}\n", 10)
	f, err := bibtex.ParseWithOptions([]byte(src), bibtex.ParseOptions{MaxDiagnostics: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 10 || len(f.Diagnostics) != 4 || f.Diagnostics[3].Code != "diagnostic_limit" {
		t.Fatalf("diagnostics %+v", f.Diagnostics)
	}
}

func TestReadAndReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CITATION.bib")
	if err := os.WriteFile(path, []byte(software), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := bibtex.ReadFile(path, bibtex.ParseOptions{})
	if err != nil || len(f.Entries) != 1 {
		t.Fatalf("ReadFile: %v %+v", err, f)
	}
	if _, err := bibtex.Read(strings.NewReader(software), bibtex.ParseOptions{MaxBytes: 10}); !errors.Is(err, bibtex.ErrLimit) {
		t.Fatalf("Read limit: %v", err)
	}
	if _, err := bibtex.Read(nil, bibtex.ParseOptions{}); !errors.Is(err, bibtex.ErrOptions) {
		t.Fatalf("nil reader: %v", err)
	}
	if _, err := bibtex.ReadFile(filepath.Join(t.TempDir(), "missing.bib"), bibtex.ParseOptions{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestParseCopiesInput(t *testing.T) {
	data := []byte(software)
	f, err := bibtex.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for i := range data {
		data[i] = 'x'
	}
	if f.Entries[0].Key != "doe_2024_example" || f.Entries[0].Get("version") != "v1.2.3" {
		t.Fatal("result shares the input buffer")
	}
}

func codes(diags []bibtex.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func entryKeys(f *bibtex.File) []string {
	var out []string
	for _, e := range f.Entries {
		out = append(out, e.Key)
	}
	return out
}
