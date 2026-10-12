// Command read prints the entries of a BibTeX file, or of a README with
// -extract, as JSON. Supply - to read a Git blob from standard input.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/git-pkgs/bibtex"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type entry struct {
	Type    string        `json:"type"`
	Key     string        `json:"key"`
	Title   string        `json:"title,omitempty"`
	Authors []bibtex.Name `json:"authors,omitempty"`
	DOI     string        `json:"doi,omitempty"`
	URL     string        `json:"url,omitempty"`
	Line    int           `json:"line"`
}

func run() error {
	extract := flag.Bool("extract", false, "find entries embedded in a README or other document")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("usage: read [-extract] <file|->")
	}
	var data []byte
	var err error
	if flag.Arg(0) == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(flag.Arg(0))
	}
	if err != nil {
		return err
	}
	var f *bibtex.File
	if *extract {
		f, err = bibtex.Extract(data, bibtex.ParseOptions{})
	} else {
		f, err = bibtex.Parse(data)
	}
	if err != nil {
		return err
	}
	result := struct {
		Entries     []entry             `json:"entries"`
		Diagnostics []bibtex.Diagnostic `json:"diagnostics,omitempty"`
	}{Entries: []entry{}, Diagnostics: f.Diagnostics}
	for i := range f.Entries {
		e := &f.Entries[i]
		result.Entries = append(result.Entries, entry{
			Type: e.Type, Key: e.Key, Title: e.Text("title"), Authors: e.Authors(),
			DOI: e.Get("doi"), URL: e.Get("url"), Line: e.Line,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}
