package bibtex

import "bytes"

// fencedBlocks returns the content byte ranges of Markdown fenced code
// blocks. An unclosed fence runs to the end of the input, as in CommonMark.
func fencedBlocks(data []byte) [][2]int {
	if !bytes.Contains(data, []byte("```")) && !bytes.Contains(data, []byte("~~~")) {
		return nil
	}
	var blocks [][2]int
	var fence byte
	var fenceLen, contentStart int
	for lineStart := 0; lineStart < len(data); {
		lineEnd := len(data)
		if i := bytes.IndexByte(data[lineStart:], '\n'); i >= 0 {
			lineEnd = lineStart + i + 1
		}
		c, n, rest := fenceMarker(data[lineStart:lineEnd])
		switch {
		case fence == 0 && n >= 3 && (c != '`' || bytes.IndexByte(rest, '`') < 0):
			fence, fenceLen, contentStart = c, n, lineEnd
		case fence != 0 && c == fence && n >= fenceLen && len(bytes.TrimSpace(rest)) == 0:
			blocks = append(blocks, [2]int{contentStart, lineStart})
			fence = 0
		}
		lineStart = lineEnd
	}
	if fence != 0 {
		blocks = append(blocks, [2]int{contentStart, len(data)})
	}
	return blocks
}

// fenceMarker reports the fence character and run length opening line, with
// up to three spaces of indentation, and the text after the run.
func fenceMarker(line []byte) (byte, int, []byte) {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	if i >= len(line) || (line[i] != '`' && line[i] != '~') {
		return 0, 0, nil
	}
	c := line[i]
	n := 0
	for i+n < len(line) && line[i+n] == c {
		n++
	}
	return c, n, line[i+n:]
}

// looksLikeEntry reports whether data at the @ begins "@type{key," or the
// keyless "@type{field =", with an alphabetic type and a key free of
// characters that cannot appear in one. This rejects decorators and
// annotations such as @dataclass(frozen=True).
func looksLikeEntry(data []byte, at int) bool {
	i := at + 1
	skip := func() {
		for i < len(data) && (data[i] == ' ' || data[i] == '\t') {
			i++
		}
	}
	skip()
	s := i
	for i < len(data) && (data[i]|0x20 >= 'a' && data[i]|0x20 <= 'z') {
		i++
	}
	if i == s {
		return false
	}
	skip()
	if i >= len(data) || (data[i] != '{' && data[i] != '(') {
		return false
	}
	braced := data[i] == '{'
	i++
	for i < len(data) && isSpace(data[i]) {
		i++
	}
	for i < len(data) && !isSpace(data[i]) && !bytes.ContainsRune([]byte(",{}()\"'#%=\\"), rune(data[i])) {
		i++
	}
	skip()
	if i >= len(data) {
		return false
	}
	// A keyless entry starts with a field; only accept that form with braces,
	// since annotations like @Retry(times = 3) use parentheses.
	if data[i] == '=' {
		return braced
	}
	if data[i] != ',' {
		return false
	}
	// After the key, require a field assignment or the closing delimiter so
	// that macro calls like @variable(model, x >= 0) are rejected.
	i++
	for i < len(data) && (isSpace(data[i]) || data[i] == '>') {
		i++
	}
	if i >= len(data) || data[i] == '}' || data[i] == ')' {
		return true
	}
	s = i
	for i < len(data) && isIdentChar(data[i]) {
		i++
	}
	skip()
	return i > s && i < len(data) && data[i] == '='
}

// entryLineStart is atLineStart that also accepts list markers, quote
// markers and an inline code backtick before the @, as READMEs use.
func entryLineStart(data []byte, at int) bool {
	for i := at - 1; i >= 0; i-- {
		switch data[i] {
		case '\n':
			return true
		case ' ', '\t', '`', '>', '-', '*', '+':
		default:
			return false
		}
	}
	return true
}

// inBlockquote reports whether a > marker precedes the @ on its line.
func inBlockquote(data []byte, at int) bool {
	for i := at - 1; i >= 0 && data[i] != '\n'; i-- {
		if data[i] == '>' {
			return true
		}
	}
	return false
}

// stripQuoteMarkers removes > markers that begin continuation lines of a
// value. It reuses b.
func stripQuoteMarkers(b []byte) []byte {
	out := b[:0]
	lineStart := false
	for _, c := range b {
		switch {
		case c == '\n':
			lineStart = true
		case lineStart && c == '>':
			continue
		case lineStart && c != ' ' && c != '\t':
			lineStart = false
		}
		out = append(out, c)
	}
	return out
}

// lineIndent returns the number of spaces and tabs before the @ at at, or 0
// when anything else precedes it on its line.
func lineIndent(data []byte, at int) int {
	n := 0
	for i := at - 1; i >= 0 && data[i] != '\n'; i-- {
		if data[i] != ' ' && data[i] != '\t' {
			return 0
		}
		n++
	}
	return n
}

// dedentEnd bounds an entry in an indented code block, such as a
// reStructuredText code directive. It returns the start of the first
// non-blank line after at indented less than indent, or the end of that
// line when it starts with a closing delimiter.
func dedentEnd(data []byte, at, indent int) int {
	i := bytes.IndexByte(data[at:], '\n')
	if i < 0 {
		return len(data)
	}
	for start := at + i + 1; start < len(data); {
		end := len(data)
		if j := bytes.IndexByte(data[start:], '\n'); j >= 0 {
			end = start + j + 1
		}
		n := 0
		for start+n < end && (data[start+n] == ' ' || data[start+n] == '\t') {
			n++
		}
		if start+n < end && !isSpace(data[start+n]) && n < indent {
			if data[start+n] == '}' || data[start+n] == ')' {
				return end
			}
			return start
		}
		start = end
	}
	return len(data)
}
