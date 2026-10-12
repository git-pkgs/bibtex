package bibtex

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// ParseOptions bounds work for untrusted inputs. Zero fields select defaults.
type ParseOptions struct {
	MaxBytes         int64
	MaxEntries       int
	MaxValueBytes    int
	MaxExpandedBytes int
	MaxDiagnostics   int
}

const (
	defaultMaxBytes         = 16 << 20
	defaultMaxEntries       = 200_000
	defaultMaxValueBytes    = 1 << 20
	defaultMaxExpandedBytes = 64 << 20
	defaultMaxDiagnostics   = 100
)

func (o ParseOptions) defaults() (ParseOptions, error) {
	if o.MaxBytes < 0 || o.MaxBytes == math.MaxInt64 || o.MaxEntries < 0 ||
		o.MaxValueBytes < 0 || o.MaxExpandedBytes < 0 || o.MaxDiagnostics < 0 {
		return o, failure(ErrOptions, "options", "limits must be positive finite values", Position{})
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = defaultMaxBytes
	}
	if o.MaxEntries == 0 {
		o.MaxEntries = defaultMaxEntries
	}
	if o.MaxValueBytes == 0 {
		o.MaxValueBytes = defaultMaxValueBytes
	}
	if o.MaxExpandedBytes == 0 {
		o.MaxExpandedBytes = defaultMaxExpandedBytes
	}
	if o.MaxDiagnostics == 0 {
		o.MaxDiagnostics = defaultMaxDiagnostics
	}
	return o, nil
}

// Parse reads a BibTeX database with default limits.
func Parse(data []byte) (*File, error) { return ParseWithOptions(data, ParseOptions{}) }

// ParseWithOptions reads a BibTeX database. Text outside entries is ignored, as
// BibTeX does. Malformed entries produce diagnostics; an error is returned
// only for invalid options, unsupported encodings, and exceeded limits.
func ParseWithOptions(data []byte, opts ParseOptions) (*File, error) {
	p, err := newParser(data, opts)
	if err != nil {
		return nil, err
	}
	if err := p.parseAll(); err != nil {
		return nil, err
	}
	return p.file, nil
}

// Extract finds BibTeX entries embedded in other text, such as a README. An
// entry is recognised where a line begins with "@type{key," or "@type(key,"
// after optional indentation. Inside a Markdown fenced code block, an entry
// cannot extend past the closing fence.
func Extract(data []byte, opts ParseOptions) (*File, error) {
	p, err := newParser(data, opts)
	if err != nil {
		return nil, err
	}
	if err := p.extract(); err != nil {
		return nil, err
	}
	return p.file, nil
}

// Read parses one bounded database from r and leaves r open.
func Read(r io.Reader, opts ParseOptions) (*File, error) {
	opts, err := opts.defaults()
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, failure(ErrOptions, "reader", "reader is nil", Position{})
	}
	data, err := io.ReadAll(io.LimitReader(r, opts.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("bibtex: read: %w", err)
	}
	return ParseWithOptions(data, opts)
}

// ReadFile parses the database at path.
func ReadFile(path string, opts ParseOptions) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("bibtex: open: %w", err)
	}
	file, readErr := Read(f, opts)
	if err := errors.Join(readErr, f.Close()); err != nil {
		return nil, err
	}
	return file, nil
}
