package bibtex

import (
	"bytes"
	"sort"
	"strings"
	"unicode/utf8"
)

type parser struct {
	data   []byte
	pos    int
	end    int
	opts   ParseOptions
	file   *File
	macros map[string]string
	keys   map[string]struct{}
	lines  []int
	// colLine, colOffset and colCount cache the last column computed.
	colLine, colOffset, colCount int
	latin                        bool
	err                          error
	buf                          []byte
	// expanded counts value bytes produced so far, including macro copies.
	expanded int
	// blockquote is set while extracting an entry that starts inside a
	// Markdown blockquote, so line-leading > markers are skipped.
	blockquote bool
}

var monthMacros = map[string]string{
	"jan": "January", "feb": "February", "mar": "March", "apr": "April",
	"may": "May", "jun": "June", "jul": "July", "aug": "August",
	"sep": "September", "oct": "October", "nov": "November", "dec": "December",
}

func newParser(data []byte, opts ParseOptions) (*parser, error) {
	opts, err := opts.defaults()
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > opts.MaxBytes {
		return nil, failure(ErrLimit, "byte_limit", "input exceeds byte limit", Position{})
	}
	if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) || bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		return nil, failure(ErrUnsupported, "encoding", "UTF-16 input is not supported", Position{})
	}
	p := &parser{data: data, end: len(data), opts: opts, file: &File{Encoding: "utf-8"}}
	if bytes.HasPrefix(data, []byte("\xEF\xBB\xBF")) {
		p.pos = 3
	}
	if !utf8.Valid(data) {
		p.latin = true
		p.file.Encoding = "windows-1252"
	}
	return p, nil
}

func (p *parser) parseAll() error {
	for p.err == nil && p.pos < p.end {
		i := bytes.IndexByte(p.data[p.pos:p.end], '@')
		if i < 0 {
			break
		}
		p.pos += i
		p.command()
	}
	return p.err
}

func (p *parser) extract() error {
	fences := fencedBlocks(p.data)
	covered := 0
	// blockEnds caches dedentEnd by indent; an entry before the cached end
	// shares it, which keeps many entries in one block linear.
	blockEnds := map[int]int{}
	for p.err == nil && p.pos < len(p.data) {
		i := bytes.IndexByte(p.data[p.pos:], '@')
		if i < 0 {
			break
		}
		at := p.pos + i
		p.pos = at + 1
		if at < covered || !entryLineStart(p.data, at) || !looksLikeEntry(p.data, at) {
			continue
		}
		p.end = len(p.data)
		if k := sort.Search(len(fences), func(k int) bool { return fences[k][1] > at }); k < len(fences) && at >= fences[k][0] {
			p.end = fences[k][1]
		} else if indent := lineIndent(p.data, at); indent > 0 {
			if end, ok := blockEnds[indent]; ok && at < end {
				p.end = end
			} else {
				p.end = dedentEnd(p.data, at, indent)
				blockEnds[indent] = p.end
			}
		}
		p.pos = at
		p.blockquote = inBlockquote(p.data, at)
		p.command()
		if p.pos <= at {
			p.pos = at + 1
		}
		covered = p.pos
	}
	p.end = len(p.data)
	return p.err
}

// command parses the construct starting at the @ under p.pos.
func (p *parser) command() {
	start := p.pos
	p.pos++
	p.skipSpace()
	typ := strings.ToLower(p.str(p.ident()))
	if typ == "" {
		return
	}
	p.skipSpace()
	if p.pos >= p.end || (p.data[p.pos] != '{' && p.data[p.pos] != '(') {
		if typ == "comment" {
			p.skipLine()
		}
		return
	}
	closer := byte('}')
	if p.data[p.pos] == '(' {
		closer = ')'
	}
	p.pos++
	switch typ {
	case "comment":
		p.skipComment(start, closer)
	case "preamble":
		if v, ok := p.value(closer, true); ok {
			p.file.Preambles = append(p.file.Preambles, v)
		}
		p.expectClose(closer)
	case "string":
		p.stringDef(closer)
	default:
		p.entry(start, typ, closer)
	}
}

func (p *parser) entry(start int, typ string, closer byte) {
	if len(p.file.Entries) >= p.opts.MaxEntries {
		p.fail(failure(ErrLimit, "entry_limit", "input exceeds entry limit", p.position(start)))
		return
	}
	e := Entry{Type: typ, Position: p.position(start), Start: start}
	p.skipSpace()
	keyStart := p.pos
	for p.pos < p.end && !isKeyEnd(p.data[p.pos], closer) {
		p.pos++
	}
	key := p.data[keyStart:p.pos]
	p.skipSpace()
	switch {
	case p.peek() == '=':
		p.diag("missing_key", "entry has no citation key", start)
		p.pos = keyStart
	case len(key) == 0:
		p.diag("missing_key", "entry has no citation key", start)
		if p.peek() == ',' {
			p.pos++
		}
	default:
		e.Key = p.str(key)
		if p.peek() == ',' {
			p.pos++
		} else if p.peek() != int(closer) {
			p.diag("missing_comma", "expected comma after citation key", p.pos)
		}
	}
	p.fields(&e, closer)
	e.End = p.pos
	if e.Key != "" {
		folded := strings.ToLower(e.Key)
		if p.keys == nil {
			p.keys = map[string]struct{}{}
		}
		if _, dup := p.keys[folded]; dup {
			p.diag("duplicate_key", "duplicate citation key "+e.Key, start)
		}
		p.keys[folded] = struct{}{}
	}
	if p.err == nil {
		p.file.Entries = append(p.file.Entries, e)
	}
}

func (p *parser) fields(e *Entry, closer byte) {
	seen := map[string]struct{}{}
	// recovering suppresses repeated diagnostics while skipping junk.
	recovering := false
	for p.err == nil {
		p.skipSpace()
		if p.pos >= p.end {
			p.diag("unterminated_entry", "entry is not closed", e.Start)
			return
		}
		switch c := p.data[p.pos]; {
		case c == closer:
			p.pos++
			return
		case c == '@' && atLineStart(p.data, p.pos):
			p.diag("unterminated_entry", "entry is not closed before the next entry", e.Start)
			return
		case c == ',':
			p.pos++
		default:
			recovering = p.field(e, closer, seen, recovering)
		}
	}
}

// field parses one name = value pair, or skips junk where one was expected.
// It returns whether the parser is still recovering from junk.
func (p *parser) field(e *Entry, closer byte, seen map[string]struct{}, recovering bool) bool {
	c := p.data[p.pos]
	nameStart := p.pos
	name := strings.ToLower(p.str(p.ident()))
	if name == "" {
		if !recovering {
			p.diag("unexpected_character", "expected field name", p.pos)
		}
		if c == '{' || c == '"' || c == '=' || c == '#' {
			p.skipField(closer)
		} else {
			p.pos++
		}
		return true
	}
	p.skipSpace()
	if p.peek() != '=' {
		if !recovering {
			p.diag("missing_equals", "expected = after field "+name, p.pos)
		}
		// Junk before a field name, such as "; title =", resumes at the name.
		if p.pos >= p.end || !isIdentChar(p.data[p.pos]) {
			p.skipField(closer)
		}
		return true
	}
	p.pos++
	v, _ := p.value(closer, true)
	if p.err != nil {
		return false
	}
	if _, dup := seen[name]; dup {
		p.diag("duplicate_field", "duplicate field "+name, nameStart)
	}
	seen[name] = struct{}{}
	e.Fields = append(e.Fields, Field{Name: name, Value: v, Position: p.position(nameStart)})
	p.skipSpace()
	if p.pos < p.end && isIdentChar(p.data[p.pos]) {
		p.diag("missing_comma", "expected comma between fields", p.pos)
	}
	return false
}

func (p *parser) stringDef(closer byte) {
	p.skipSpace()
	nameStart := p.pos
	name := strings.ToLower(p.str(p.ident()))
	p.skipSpace()
	if name == "" || p.peek() != '=' {
		p.diag("invalid_string", "expected name = value in @string", nameStart)
		p.skipComment(nameStart, closer)
		return
	}
	p.pos++
	v, _ := p.value(closer, false)
	if p.err != nil {
		return
	}
	if p.macros == nil {
		p.macros = map[string]string{}
	}
	p.macros[name] = v
	p.file.Strings = append(p.file.Strings, Field{Name: name, Value: v, Position: p.position(nameStart)})
	p.expectClose(closer)
}

// value parses one or more pieces joined by #. It returns false when the
// value was missing or unterminated. @string values are not trimmed, so
// "Proc. " # x keeps its space.
func (p *parser) value(closer byte, trim bool) (string, bool) {
	p.buf = p.buf[:0]
	ok := true
	for p.err == nil {
		p.skipSpace()
		if p.pos >= p.end {
			p.diag("missing_value", "expected value", p.pos)
			return "", false
		}
		c := p.data[p.pos]
		switch {
		case c == '{':
			p.pos++
			ok = p.delimited(false) && ok
		case c == '"':
			p.pos++
			ok = p.delimited(true) && ok
		case c >= '0' && c <= '9':
			s := p.pos
			for p.pos < p.end && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
				p.pos++
			}
			p.buf = append(p.buf, p.data[s:p.pos]...)
		case isIdentChar(c):
			s := p.pos
			name := strings.ToLower(p.str(p.ident()))
			if m, found := p.macros[name]; found {
				p.buf = append(p.buf, m...)
			} else if m, found := monthMacros[name]; found {
				p.buf = append(p.buf, m...)
			} else {
				p.diag("undefined_macro", "undefined macro "+name, s)
				p.buf = append(p.buf, p.data[s:p.pos]...)
			}
		default:
			if c != closer && c != ',' {
				p.diag("unexpected_character", "expected value", p.pos)
			} else {
				p.diag("missing_value", "expected value", p.pos)
			}
			return "", false
		}
		if len(p.buf) > p.opts.MaxValueBytes {
			p.fail(failure(ErrLimit, "value_limit", "value exceeds byte limit", p.position(p.pos)))
			return "", false
		}
		p.skipSpace()
		if p.peek() != '#' {
			break
		}
		p.pos++
	}
	if p.blockquote {
		p.buf = stripQuoteMarkers(p.buf)
	}
	v := p.decode(collapseSpace(p.buf, trim))
	p.expanded += len(v)
	if p.expanded > p.opts.MaxExpandedBytes {
		p.fail(failure(ErrLimit, "expansion_limit", "expanded values exceed byte limit", p.position(p.pos)))
		return "", false
	}
	return v, ok
}

// delimited appends a {braced} or "quoted" piece after its opening delimiter.
// Braces count literally, as in BibTeX, so \{ must also be balanced. A piece
// ends early at a line that starts like a new entry, so an unclosed brace
// cannot swallow the rest of the input, and recovery stays linear when many
// entries are unclosed.
func (p *parser) delimited(quoted bool) bool {
	start := p.pos
	depth := 0
	if !quoted {
		depth = 1
	}
	stops := "{}@"
	if quoted {
		stops = "{}@\""
	}
	for p.pos < p.end {
		i := bytes.IndexAny(p.data[p.pos:p.end], stops)
		if i < 0 {
			p.pos = p.end
			break
		}
		p.pos += i
		switch p.data[p.pos] {
		case '@':
			if atLineStart(p.data, p.pos) && looksLikeEntry(p.data[:p.end], p.pos) {
				p.diag("unterminated_value", "value is not closed before the next entry", start-1)
				p.buf = append(p.buf, p.data[start:p.pos]...)
				return false
			}
		case '{':
			depth++
		case '}':
			if depth == 0 {
				p.diag("unbalanced_braces", "unexpected closing brace", p.pos)
				break
			}
			depth--
			if depth == 0 && !quoted {
				p.buf = append(p.buf, p.data[start:p.pos]...)
				p.pos++
				return true
			}
		case '"':
			if depth == 0 {
				p.buf = append(p.buf, p.data[start:p.pos]...)
				p.pos++
				return true
			}
		}
		p.pos++
	}
	p.diag("unterminated_value", "value is not closed", start-1)
	p.buf = append(p.buf, p.data[start:p.pos]...)
	return false
}

func (p *parser) skipComment(start int, closer byte) {
	depth := 0
	for p.pos < p.end {
		switch p.data[p.pos] {
		case '{':
			depth++
		case '}':
			if depth == 0 && closer == '}' {
				p.pos++
				return
			}
			if depth > 0 {
				depth--
			}
		case ')':
			if depth == 0 && closer == ')' {
				p.pos++
				return
			}
		}
		p.pos++
	}
	p.diag("unterminated_comment", "construct is not closed", start)
}

func (p *parser) expectClose(closer byte) {
	p.skipSpace()
	if p.peek() == int(closer) {
		p.pos++
		return
	}
	p.diag("unterminated_entry", "expected closing delimiter", p.pos)
	p.skipComment(p.pos, closer)
}

// skipField recovers from a malformed field by advancing to the next comma,
// closing delimiter or line starting with @, outside braces.
func (p *parser) skipField(closer byte) {
	start := p.pos
	depth := 0
	for p.pos < p.end {
		c := p.data[p.pos]
		switch {
		case c == '{':
			depth++
		case c == '}' && depth > 0:
			depth--
		case depth == 0 && (c == ',' || c == closer):
			if p.pos == start {
				p.pos++
			}
			return
		case c == '@' && p.pos > start && atLineStart(p.data, p.pos):
			return
		}
		p.pos++
	}
}

func (p *parser) skipLine() {
	if i := bytes.IndexByte(p.data[p.pos:p.end], '\n'); i >= 0 {
		p.pos += i + 1
	} else {
		p.pos = p.end
	}
}

// skipSpace also skips Unicode spaces pasted from web pages and, when
// extracting, Markdown blockquote markers at the start of a line.
func (p *parser) skipSpace() {
	for p.pos < p.end {
		switch {
		case isSpace(p.data[p.pos]):
			p.pos++
		case p.blockquote && p.data[p.pos] == '>' && atLineStart(p.data, p.pos):
			p.pos++
		case p.latin:
			return
		default:
			n := unicodeSpace(p.data[p.pos:p.end])
			if n == 0 {
				return
			}
			p.pos += n
		}
	}
}

// unicodeSpace returns the length of a leading UTF-8 no-break space or
// ideographic space in b, or 0.
func unicodeSpace(b []byte) int {
	for _, sp := range []string{"\u00a0", "\u3000"} {
		if bytes.HasPrefix(b, []byte(sp)) {
			return len(sp)
		}
	}
	return 0
}

func (p *parser) ident() []byte {
	s := p.pos
	for p.pos < p.end && isIdentChar(p.data[p.pos]) {
		p.pos++
	}
	return p.data[s:p.pos]
}

func (p *parser) peek() int {
	if p.pos >= p.end {
		return -1
	}
	return int(p.data[p.pos])
}

func (p *parser) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

func (p *parser) diag(code, message string, offset int) {
	n := len(p.file.Diagnostics)
	switch {
	case n < p.opts.MaxDiagnostics:
		p.file.Diagnostics = append(p.file.Diagnostics, Diagnostic{Code: code, Message: message, Position: p.position(offset)})
	case n == p.opts.MaxDiagnostics:
		p.file.Diagnostics = append(p.file.Diagnostics, Diagnostic{Code: "diagnostic_limit", Message: "further diagnostics omitted"})
	}
}

func (p *parser) position(offset int) Position {
	if p.lines == nil {
		p.lines = append(p.lines, 0)
		for i := 0; ; {
			j := bytes.IndexByte(p.data[i:], '\n')
			if j < 0 {
				break
			}
			i += j + 1
			p.lines = append(p.lines, i)
		}
	}
	line := sort.SearchInts(p.lines, offset+1) - 1
	from, col := p.lines[line], 0
	// Positions are mostly requested in increasing order, so continue from
	// the previous one on the same line; long single-line inputs would
	// otherwise be quadratic.
	if p.colLine == line && p.colOffset <= offset && p.colOffset >= from {
		from, col = p.colOffset, p.colCount
	}
	prefix := p.data[from:offset]
	if p.latin {
		col += len(prefix)
	} else {
		col += utf8.RuneCount(prefix)
	}
	p.colLine, p.colOffset, p.colCount = line, offset, col
	return Position{Line: line + 1, Column: col + 1}
}

func (p *parser) str(b []byte) string { return p.decode(b) }

func (p *parser) decode(b []byte) string {
	if !p.latin {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c >= 0x80 && c < 0xA0 && cp1252[c-0x80] != 0 {
			sb.WriteRune(cp1252[c-0x80])
		} else {
			sb.WriteRune(rune(c))
		}
	}
	return sb.String()
}

var cp1252 = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

// collapseSpace replaces whitespace runs with one space, as BibTeX does for
// values, and optionally trims the ends. It reuses b.
func collapseSpace(b []byte, trim bool) []byte {
	out := b[:0]
	space := false
	for _, c := range b {
		if isSpace(c) {
			space = len(out) > 0 || !trim
			continue
		}
		if space {
			out = append(out, ' ')
			space = false
		}
		out = append(out, c)
	}
	if space && !trim {
		out = append(out, ' ')
	}
	return out
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// isIdentChar matches characters BibTeX allows in entry types, field names
// and macro names.
func isIdentChar(c byte) bool {
	if isSpace(c) || c < 0x20 || c == 0x7F {
		return false
	}
	return !strings.ContainsRune("\"#%'(),={}@\\~", rune(c))
}

func isKeyEnd(c, closer byte) bool {
	return isSpace(c) || c == ',' || c == closer || c == '=' || c == '{' || c == '}'
}

func atLineStart(data []byte, at int) bool {
	for i := at - 1; i >= 0; i-- {
		switch data[i] {
		case '\n':
			return true
		case ' ', '\t':
		default:
			return false
		}
	}
	return true
}
