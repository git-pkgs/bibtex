# bibtex

A Go library for reading BibTeX that uses only the standard library. It parses `.bib` files such as `CITATION.bib` and BibTeX entries embedded in READMEs, splits author lists using BibTeX's name rules, and converts TeX markup to plain Unicode text. It is built for scanning untrusted input in bulk, such as repository histories or archive blobs: malformed entries produce diagnostics and parsing continues past them. Configurable limits bound the work, and the test suite checks time and memory on pathological inputs.

## Installation

```bash
go get github.com/git-pkgs/bibtex
```

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/git-pkgs/bibtex"
)

func main() {
	f, err := bibtex.ReadFile("CITATION.bib", bibtex.ParseOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range f.Entries {
		fmt.Println(e.Type, e.Key, e.Text("title"), e.Get("doi"))
		for _, a := range e.Authors() {
			fmt.Printf("  %s | %s | %s | %s\n", a.First, a.Von, a.Last, a.Jr)
		}
	}
	for _, d := range f.Diagnostics {
		fmt.Println(d.Line, d.Code, d.Message)
	}
}
```

Use `Parse(data)` for bytes from a Git blob or another source and `ParseWithOptions(data, opts)` to set limits. `Read(reader, opts)` reads from an `io.Reader` and leaves it open. Results store copies of their strings, so callers may reuse input buffers once a call returns. `go run ./examples/read path/to/CITATION.bib` runs a version of this example; `-extract` reads a README and `-` reads a Git blob from standard input.

`Get` returns a field with macros expanded and `#` concatenation applied but TeX markup and inner braces intact (`{GPU}`, `M{\"u}ller`), while `Text` also runs it through `PlainText` to give `GPU` and `Müller`. For biblatex's verbatim fields (`url`, `doi`, `eprint`, `file` and `pdf`), `Text` removes braces, `\url{}` and escapes such as `\_` and leaves `~` and `--` as written. Field names and entry types are lowercased, and citation keys are returned with their original case. Each entry stores its line, column and byte range in the input.

### READMEs

`Extract(data, opts)` returns entries inside another document, with positions relative to the whole document. An entry is recognised where a line starts with `@type{key,`, which covers Markdown fenced code blocks (with an optional language tag), blockquotes, list items, inline code and reStructuredText code directives. Decorators and macros such as `@dataclass(frozen=True)` or Julia's `@variable(model, x >= 0)` are skipped. An unclosed entry stops at the end of its fenced block or, for indented blocks, at the first line indented less than the `@`.

### Names

`ParseNames` splits a value on `and` outside braces and applies BibTeX's rules for `First von Last`, `von Last, First` and `von Last, Jr, First`, including the brace and accent rules that assign a word to the von part. A fully braced name such as `{World Health Organization}` is returned whole in `Last` with `Literal` set, which is how BibTeX files mark organisations, and `and others` becomes a `Name` with `Others` set. On 5,248 names collected from `CITATION.bib` files and READMEs, the split matched pybtex except for comma-separated lists such as `Jifeng Dai, Yi Li, Kaiming He`, which this library reads as several names.

The comma-list heuristic also reads the ambiguous `de la Cruz, Maria Elena, Jose Luis` as three names, although BibTeX reads it as one name with a Jr part. `Get("author")` retains the name syntax, including braces and commas, after macro expansion and whitespace folding, so callers can use a different name interpretation without parsing the file again.

### Errors

Missing commas, missing keys, unclosed values, undefined macros, duplicate keys and similar problems are reported in `File.Diagnostics` with a code and position. An unclosed value ends at the next line that starts like an entry, and parsing resumes from that entry. Invalid options, UTF-16 input and exceeded limits return an error, matched with `errors.Is` against `ErrOptions`, `ErrUnsupported` and `ErrLimit` respectively; `errors.As` extracts the `*bibtex.Error` diagnostic.

Duplicate citation keys are matched without regard to case and produce a `duplicate_key` diagnostic. All entries are retained in source order; callers choose which occurrence to use.

## Compatibility

The parser follows BibTeX's grammar: entries in braces or parentheses, quoted and braced values with literally counted braces, numbers, `@string` macros with case-insensitive names, the predefined month macros, `#` concatenation, `@preamble`, and `@comment` in both its braced and line forms. Text outside entries is ignored, and whitespace runs inside values collapse to one space.

Some behaviour is more forgiving than BibTeX because `CITATION.bib` files and README entries are often hand-written. An undefined macro's name is used as its value (`booktitle = CCS` gives `CCS`) and reported as a diagnostic. A name with two or more commas whose parts are each at least two words, such as `Jifeng Dai, Yi Li, Kaiming He`, is read as a list of names, where BibTeX rejects it. Input with invalid UTF-8 is decoded as Windows-1252 and reported through `File.Encoding`. Non-breaking and ideographic spaces count as whitespace between tokens.

`PlainText` composes accent commands into precomposed characters using a table generated from Unicode 17.0.0, with a combining mark as the fallback where Unicode has no precomposed form. It replaces known symbols such as `\o`, `\ss` and `\LaTeX` and converts `--`, `---` and TeX quotes. Other commands are dropped and their arguments retained; math between `$` signs is returned unchanged.

`Text("title")` preserves math too: `An $O(n^2)$ algorithm` keeps its dollar signs and TeX expression. Callers matching titles against services such as OpenAlex need to normalise math themselves.

Out of scope: user-defined `@preamble` macros, output formatting, BibLaTeX data model checks and per-type required field validation.

## Limits

Default limits are 16 MiB of input, 200,000 entries, 1 MiB per value, 64 MiB of values in total including macro expansion, and 100 diagnostics. `ParseOptions` accepts other finite limits; a zero value uses the default and a negative value returns `ErrOptions`. The expansion limit stops macros defined in terms of each other from doubling the value size on each expansion.

## Testing

```bash
go test ./...
go test -race ./...
go test -fuzz=FuzzParse -fuzztime=30s
go test -bench=. -run=^$
```

Tests run offline against checked-in fixtures. `testdata/real-world` contains 24 `CITATION.bib` files and READMEs from GitHub repositories, with their sources and Git blob IDs in `downloads.json` and hand-reviewed expectations in `manifest.json`. `go run ./internal/generate UnicodeData.txt > compose_generated.go` regenerates the accent table from the pinned Unicode 17.0.0 data file.

## License

[MIT](LICENSE). See [NOTICE](NOTICE) for the Unicode data license and fixture sources.
