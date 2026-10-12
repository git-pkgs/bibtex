package bibtex_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/bibtex"
)

func TestExtractReadme(t *testing.T) {
	readme := "# Project\n\n" +
		"Contact us at team@example.org or @maintainer.\n\n" +
		"```python\n" +
		"@dataclass(frozen=True)\n" +
		"class Config: pass\n\n" +
		"@app.route(\"/\")\n" +
		"@retry(times = 3)\n" +
		"def index(): pass\n" +
		"```\n\n" +
		"```julia\n" +
		"@variable(model, x[periods] >= 0)\n" +
		"@constraint(model, sum(x) <= 4)\n" +
		"```\n\n" +
		"## Citation\n\n" +
		"```bibtex\n" +
		"@article{fenced,\n  title = {Fenced},\n  author = {Doe, Jane}\n}\n" +
		"```\n\n" +
		"```\n" +
		"@misc{untagged, title = {Untagged}}\n" +
		"```\n\n" +
		"> @misc{quoted,\n>   title = {Quoted\n>   across lines},\n>   author = {Roe, R.}\n> }\n\n" +
		"- `@software{bullet, title = {Bullet}}`\n\n" +
		"Cite as:\n\n" +
		".. code:: bibtex\n\n" +
		"    @inproceedings{indented,\n      title = {Indented}\n    }\n\n" +
		"@article{\n  title = {Keyless}\n}\n"
	f, err := bibtex.Extract([]byte(readme), bibtex.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"fenced": "Fenced", "untagged": "Untagged", "quoted": "Quoted across lines",
		"bullet": "Bullet", "indented": "Indented", "": "Keyless",
	}
	got := map[string]string{}
	for _, e := range f.Entries {
		got[e.Key] = e.Get("title")
		if !strings.HasPrefix(readme[e.Start:], "@") {
			t.Errorf("%s: span starts %q", e.Key, readme[e.Start:e.Start+5])
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries %v", got)
	}
	if a := f.Entries[2].Authors(); len(a) != 1 || a[0].Last != "Roe" {
		t.Fatalf("blockquote authors %+v", a)
	}
	if e := f.Entries[0]; e.Line != 22 || e.Column != 1 {
		t.Fatalf("position relative to README: %+v", e.Position)
	}
}

func TestExtractBoundsBrokenEntries(t *testing.T) {
	readme := "```bibtex\n@misc{fenced, title = {Unclosed\n```\n\nProse that is not part of the entry.\n\n" +
		"    @misc{indented, title = {Unclosed\n\nMore prose, with = signs.\n\n" +
		"@misc{after, title = {After}}\n"
	f, err := bibtex.Extract([]byte(readme), bibtex.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if keys := entryKeys(f); !reflect.DeepEqual(keys, []string{"fenced", "indented", "after"}) {
		t.Fatalf("keys %v", keys)
	}
	if v := f.Entries[0].Get("title"); v != "Unclosed" {
		t.Fatalf("fenced value ran past fence: %q", v)
	}
	if v := f.Entries[1].Get("title"); v != "Unclosed" {
		t.Fatalf("indented value ran past block: %q", v)
	}
	for _, d := range f.Diagnostics {
		if d.Code != "unterminated_value" && d.Code != "unterminated_entry" {
			t.Fatalf("unexpected diagnostic %+v", d)
		}
	}
}

func TestExtractIgnoresNonEntries(t *testing.T) {
	for _, src := range []string{
		"",
		"no entries here",
		"email me@example.org",
		"    @Override\n    public void run() {}",
		"@SuppressWarnings({\"a\", \"b\"})",
		"@pytest.mark.parametrize(\"a,b\", [(1, 2)])",
		"@Component({\n  selector: 'app',\n})",
		"text @misc{inline, title = {Mid-line}}",
	} {
		f, err := bibtex.Extract([]byte(src), bibtex.ParseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(f.Entries) != 0 || len(f.Diagnostics) != 0 {
			t.Errorf("%q: %+v", src, f)
		}
	}
}

func TestExtractBOM(t *testing.T) {
	src := "\ufeff  @misc{k, title = {Example}}"
	f, err := bibtex.Extract([]byte(src), bibtex.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 1 || f.Entries[0].Get(titleField) != "Example" {
		t.Fatalf("entries %+v", f.Entries)
	}
	e := f.Entries[0]
	if e.Start != len("\ufeff  ") || e.End != len(src) || e.Line != 1 || e.Column != 4 {
		t.Fatalf("position %+v, span %d-%d", e.Position, e.Start, e.End)
	}
	src = "\ufeff```bibtex\n@misc{k, title = {Unclosed\n```\nUnrelated prose }"
	f, err = bibtex.Extract([]byte(src), bibtex.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 1 || f.Entries[0].Get(titleField) != "Unclosed" {
		t.Fatalf("BOM-prefixed fence: %+v", f.Entries)
	}
}

func TestExtractBoundsQuotedFences(t *testing.T) {
	for _, prefix := range []string{"> ", "> > ", "  > "} {
		for _, fence := range []string{"```", "~~~"} {
			src := prefix + fence + "bibtex\n" +
				prefix + "@misc{broken, title = {Unclosed\n" +
				prefix + fence + "\n\nUnrelated prose with a brace }\n\n" +
				"@misc{after, title = {After}}\n"
			f, err := bibtex.Extract([]byte(src), bibtex.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if got := entryKeys(f); !reflect.DeepEqual(got, []string{"broken", "after"}) {
				t.Fatalf("prefix=%q fence=%q: keys %v", prefix, fence, got)
			}
			if got := f.Entries[0].Get(titleField); got != "Unclosed" {
				t.Errorf("prefix=%q fence=%q: title %q", prefix, fence, got)
			}
			if f.Entries[0].End != strings.Index(src, prefix+fence+"\n") {
				t.Errorf("prefix=%q fence=%q: end %d", prefix, fence, f.Entries[0].End)
			}
		}
	}
}
