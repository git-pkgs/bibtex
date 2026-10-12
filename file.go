package bibtex

import "strings"

// File is the result of parsing a BibTeX database or extracting entries from
// a document. It owns its strings; the input buffer may be reused after
// parsing.
type File struct {
	Entries []Entry `json:"entries"`
	// Strings holds @string definitions in source order, with values expanded.
	Strings   []Field  `json:"strings,omitempty"`
	Preambles []string `json:"preambles,omitempty"`
	// Encoding is "utf-8", or "windows-1252" when the input was not valid
	// UTF-8 and was decoded as Windows-1252.
	Encoding    string       `json:"encoding"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Entry is one BibTeX entry such as @article{key, ...}.
type Entry struct {
	// Type is lowercased, e.g. "article".
	Type string `json:"type"`
	// Key is the citation key with its original case.
	Key    string  `json:"key"`
	Fields []Field `json:"fields"`
	Position
	// Start and End are byte offsets of the entry in the input, from the @ to
	// just after the closing delimiter.
	Start int `json:"start"`
	End   int `json:"end"`
}

// Field is a field or @string definition. Name is lowercased. Value has
// macros expanded, # concatenation applied, outer delimiters removed and
// whitespace runs collapsed; TeX markup and inner braces are retained.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Position
}

// Lookup returns the first value for a field name, matched case-insensitively.
func (e *Entry) Lookup(name string) (string, bool) {
	for i := range e.Fields {
		if strings.EqualFold(e.Fields[i].Name, name) {
			return e.Fields[i].Value, true
		}
	}
	return "", false
}

// Get returns the first value for a field name, or "" when it is absent.
func (e *Entry) Get(name string) string {
	v, _ := e.Lookup(name)
	return v
}

// verbatimFields are biblatex's verbatim fields, where ~ and -- are literal.
var verbatimFields = map[string]bool{"url": true, "doi": true, "eprint": true, "file": true, "pdf": true}

// Text returns a field converted to plain Unicode text with PlainText. The
// url, doi, eprint, file and pdf fields are returned with braces, \url{} and
// escapes such as \_ removed and other characters unchanged.
func (e *Entry) Text(name string) string {
	v := e.Get(name)
	if verbatimFields[strings.ToLower(name)] {
		return verbatim(v)
	}
	return PlainText(v)
}

var verbatimEscapes = strings.NewReplacer(`\_`, "_", `\%`, "%", `\&`, "&", `\#`, "#", `\$`, "$", `\~`, "~", "{", "", "}", "")

func verbatim(v string) string {
	v = strings.TrimSpace(v)
	for _, cmd := range []string{`\url`, `\path`, `\href`} {
		if rest, ok := strings.CutPrefix(v, cmd); ok {
			// \href{target}{text}: keep the target.
			if arg, _, found := strings.Cut(strings.TrimPrefix(strings.TrimSpace(rest), "{"), "}"); found {
				v = arg
			}
			break
		}
	}
	return strings.TrimSpace(verbatimEscapes.Replace(v))
}

// Authors returns the parsed author field.
func (e *Entry) Authors() []Name { return ParseNames(e.Get("author")) }

// Editors returns the parsed editor field.
func (e *Entry) Editors() []Name { return ParseNames(e.Get("editor")) }
