package bibtex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Name is one person or organisation from a BibTeX name list, split into
// BibTeX's four parts and converted to plain text. A name written in braces,
// such as {World Health Organization}, is returned whole in Last.
type Name struct {
	First string `json:"first,omitempty"`
	Von   string `json:"von,omitempty"`
	Last  string `json:"last,omitempty"`
	Jr    string `json:"jr,omitempty"`
	// Others marks the "and others" placeholder for omitted names.
	Others bool `json:"others,omitempty"`
}

// ParseNames splits an author or editor value on "and" outside braces and
// parses each name with BibTeX's rules for the "First von Last",
// "von Last, First" and "von Last, Jr, First" forms.
//
// Unlike BibTeX, a name with two or more commas whose parts are each at
// least two words, such as "Jifeng Dai, Yi Li, Kaiming He", is read as a
// comma-separated list of "First Last" names. READMEs often list authors
// this way, and BibTeX rejects it.
func ParseNames(s string) []Name {
	var names []Name
	for _, raw := range splitAnd(s) {
		if list := commaList(raw); list != nil {
			for _, item := range list {
				if n, ok := parseName(item); ok {
					names = append(names, n)
				}
			}
			continue
		}
		if n, ok := parseName(raw); ok {
			names = append(names, n)
		}
	}
	return names
}

// commaList returns the parts of a comma-separated name list, or nil when
// raw has fewer than two commas or a part with fewer than two words. A
// trailing comma, as in "A B, C D, and E F", is allowed.
func commaList(raw string) []string {
	const minNames, minWords = 3, 2
	parts := splitDepthZero(raw, func(c byte) bool { return c == ',' }, true)
	if len(parts) < minNames {
		return nil
	}
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	for _, p := range parts {
		if len(splitDepthZero(p, func(c byte) bool { return isSpace(c) || c == '~' }, false)) < minWords {
			return nil
		}
	}
	return parts
}

// String formats the name as "First von Last, Jr".
func (n Name) String() string {
	if n.Others {
		return "others"
	}
	var parts []string
	for _, p := range []string{n.First, n.Von, n.Last} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	s := strings.Join(parts, " ")
	if n.Jr != "" {
		s += ", " + n.Jr
	}
	return s
}

func splitAnd(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '{':
			depth++
		case c == '}':
			if depth > 0 {
				depth--
			}
		case depth == 0 && isSpace(c) && i+4 < len(s) && strings.EqualFold(s[i+1:i+4], "and") && isSpace(s[i+4]):
			out = append(out, s[start:i])
			i += 4
			start = i
		}
	}
	return append(out, s[start:])
}

func parseName(raw string) (Name, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Name{}, false
	}
	if strings.EqualFold(raw, "others") {
		return Name{Others: true}, true
	}
	parts := splitDepthZero(raw, func(c byte) bool { return c == ',' }, true)
	words := make([][]string, len(parts))
	for i, p := range parts {
		words[i] = splitDepthZero(p, func(c byte) bool { return isSpace(c) || c == '~' }, false)
	}
	var n Name
	switch commas := len(parts) - 1; commas {
	case 0:
		w := words[0]
		if len(w) == 0 {
			return Name{}, false
		}
		vonStart, vonEnd := -1, -1
		for i := 0; i < len(w)-1; i++ {
			if isLowerWord(w[i]) {
				if vonStart < 0 {
					vonStart = i
				}
				vonEnd = i
			}
		}
		if vonStart < 0 {
			n.First, n.Last = join(w[:len(w)-1]), join(w[len(w)-1:])
		} else {
			n.First, n.Von, n.Last = join(w[:vonStart]), join(w[vonStart:vonEnd+1]), join(w[vonEnd+1:])
		}
	case 1:
		n.Von, n.Last = splitVonLast(words[0])
		n.First = join(words[1])
	default:
		n.Von, n.Last = splitVonLast(words[0])
		n.Jr = join(words[1])
		var first []string
		for _, w := range words[2:] {
			first = append(first, w...)
		}
		n.First = join(first)
	}
	return n, n.First != "" || n.Von != "" || n.Last != "" || n.Jr != ""
}

func splitVonLast(w []string) (string, string) {
	end := -1
	for i := 0; i < len(w)-1; i++ {
		if isLowerWord(w[i]) {
			end = i
		}
	}
	return join(w[:end+1]), join(w[end+1:])
}

func join(words []string) string {
	return PlainText(strings.Join(words, " "))
}

// splitDepthZero splits s at separator bytes outside braces, dropping empty
// pieces unless keepEmpty is set.
func splitDepthZero(s string, sep func(byte) bool, keepEmpty bool) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '{':
			depth++
		case c == '}':
			if depth > 0 {
				depth--
			}
		case depth == 0 && sep(c):
			if piece := strings.TrimSpace(s[start:i]); piece != "" || keepEmpty {
				out = append(out, piece)
			}
			start = i + 1
		}
	}
	if piece := strings.TrimSpace(s[start:]); piece != "" || keepEmpty {
		out = append(out, piece)
	}
	return out
}

// specialLetters are control words whose own case decides a word's case.
var specialLetters = map[string]bool{
	"i": true, "j": true, "oe": true, "OE": true, "ae": true, "AE": true,
	"aa": true, "AA": true, "o": true, "O": true, "l": true, "L": true, "ss": true,
}

// isLowerWord reports whether the first letter of a word, found as BibTeX
// does, is lowercase. Braced groups not starting with a backslash are
// skipped; a group or bare command starting with one is a special
// character whose case comes from the letter it produces.
func isLowerWord(w string) bool {
	for i := 0; i < len(w); {
		c := w[i]
		switch {
		case c == '{' && i+1 < len(w) && w[i+1] == '\\':
			return specialIsLower(w[i+1:])
		case c == '\\':
			return specialIsLower(w[i:])
		case c == '{':
			depth := 0
			for ; i < len(w); i++ {
				if w[i] == '{' {
					depth++
				} else if w[i] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			i++
		default:
			r, size := utf8.DecodeRuneInString(w[i:])
			if unicode.IsLetter(r) {
				return unicode.IsLower(r)
			}
			i += size
		}
	}
	return false
}

func specialIsLower(s string) bool {
	j := 1
	for j < len(s) && isLetter(s[j]) {
		j++
	}
	if j == 1 && j < len(s) {
		j++
	}
	if name := s[1:j]; specialLetters[name] {
		return unicode.IsLower(rune(name[0]))
	}
	for _, r := range s[j:] {
		if unicode.IsLetter(r) {
			return unicode.IsLower(r)
		}
	}
	return false
}
