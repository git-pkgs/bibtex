package bibtex

import (
	"strings"
	"unicode/utf8"
)

// accentMarks maps TeX accent commands to Unicode combining characters.
var accentMarks = map[string]rune{
	"'": '\u0301', "`": '\u0300', "^": '\u0302', "\"": '\u0308', "~": '\u0303', "=": '\u0304', ".": '\u0307',
	"u": '\u0306', "v": '\u030C', "H": '\u030B', "c": '\u0327', "k": '\u0328', "r": '\u030A',
	"d": '\u0323', "b": '\u0331', "t": '\u0361',
}

var texSymbols = map[string]string{
	"o": "ø", "O": "Ø", "l": "ł", "L": "Ł", "ss": "ß", "SS": "SS", "aa": "å", "AA": "Å",
	"ae": "æ", "AE": "Æ", "oe": "œ", "OE": "Œ", "i": "ı", "j": "ȷ",
	"dh": "ð", "DH": "Ð", "th": "þ", "TH": "Þ", "ng": "ŋ", "NG": "Ŋ", "dj": "đ", "DJ": "Đ",
	"TeX": "TeX", "LaTeX": "LaTeX", "LaTeXe": "LaTeX2e", "BibTeX": "BibTeX",
	"copyright": "©", "textcopyright": "©", "textregistered": "®", "texttrademark": "™",
	"S": "§", "P": "¶", "dag": "†", "ddag": "‡", "pounds": "£", "euro": "€", "texteuro": "€",
	"textendash": "–", "textemdash": "—", "ldots": "…", "dots": "…", "textellipsis": "…",
	"textquoteleft": "‘", "textquoteright": "’", "textquotedblleft": "“", "textquotedblright": "”",
	"textdegree": "°", "textbackslash": "\\", "textasciitilde": "~", "textasciicircum": "^",
	"textunderscore": "_", "textbar": "|", "textless": "<", "textgreater": ">",
	"guillemotleft": "«", "guillemotright": "»", "textbullet": "•", "textperiodcentered": "·",
	"quad": " ", "qquad": " ", "space": " ", "par": " ", "newline": " ",
}

// maxAccentNesting bounds recursion for accent arguments such as \'{\"u}.
const maxAccentNesting = 4

// PlainText converts a BibTeX field value to plain Unicode text. Accent
// commands become precomposed characters where Unicode has one, known
// symbols are replaced, other commands are dropped while their arguments are
// kept, braces are removed and whitespace is collapsed. Math between $
// signs is kept verbatim.
func PlainText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	plain(&b, s, 0)
	return string(collapseSpace([]byte(b.String()), true))
}

func plain(b *strings.Builder, s string, nesting int) {
	for i := 0; i < len(s); {
		c := s[i]
		switch c {
		case '{', '}':
			i++
		case '\\':
			i = command(b, s, i, nesting)
		case '$':
			n := strings.IndexByte(s[i+1:], '$')
			if n < 0 {
				b.WriteString(s[i:])
				return
			}
			end := i + 1 + n + len("$")
			b.WriteString(s[i:end])
			i = end
		case '~':
			b.WriteByte(' ')
			i++
		case '-':
			switch {
			case strings.HasPrefix(s[i:], "---"):
				b.WriteString("—")
				i += 3
			case strings.HasPrefix(s[i:], "--"):
				b.WriteString("–")
				i += 2
			default:
				b.WriteByte('-')
				i++
			}
		case '`', '\'':
			if i+1 < len(s) && s[i+1] == c {
				if c == '`' {
					b.WriteString("“")
				} else {
					b.WriteString("”")
				}
				i += 2
			} else {
				b.WriteByte(c)
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
}

// command handles the control sequence at s[i] and returns the next offset.
func command(b *strings.Builder, s string, i, nesting int) int {
	if i+1 >= len(s) {
		return i + 1
	}
	j := i + 1
	var name string
	if isLetter(s[j]) {
		for j < len(s) && isLetter(s[j]) {
			j++
		}
		name = s[i+1 : j]
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
			j++
		}
	} else {
		_, size := utf8.DecodeRuneInString(s[j:])
		name = s[j : j+size]
		j += size
	}
	if mark, ok := accentMarks[name]; ok {
		arg, next := argument(s, j)
		accent(b, arg, mark, nesting)
		return next
	}
	if sym, ok := texSymbols[name]; ok {
		b.WriteString(sym)
		return j
	}
	switch name {
	case "&", "%", "$", "#", "_", "{", "}":
		b.WriteString(name)
	case "\\", " ", ",", ";", ":", "!", "\n", "\t", "\r":
		b.WriteByte(' ')
	case "url", "path":
		arg, next := argument(s, j)
		b.WriteString(arg)
		return next
	case "href":
		_, next := argument(s, j)
		return next
	}
	return j
}

// argument returns a command argument starting at s[i]: a braced group
// without its braces, a control sequence such as \i, or one character.
func argument(s string, i int) (string, int) {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= len(s) {
		return "", i
	}
	switch s[i] {
	case '{':
		depth := 0
		for j := i; j < len(s); j++ {
			switch s[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return s[i+1 : j], j + 1
				}
			}
		}
		return s[i+1:], len(s)
	case '\\':
		j := i + 1
		for j < len(s) && isLetter(s[j]) {
			j++
		}
		if j == i+1 && j < len(s) {
			_, size := utf8.DecodeRuneInString(s[j:])
			j += size
		}
		return s[i:j], j
	}
	_, size := utf8.DecodeRuneInString(s[i:])
	return s[i : i+size], i + size
}

func accent(b *strings.Builder, arg string, mark rune, nesting int) {
	var inner strings.Builder
	if nesting < maxAccentNesting {
		plain(&inner, arg, nesting+1)
	} else {
		inner.WriteString(arg)
	}
	base := strings.TrimLeft(inner.String(), " ")
	if base == "" {
		return
	}
	r, size := utf8.DecodeRuneInString(base)
	switch r {
	case 'ı':
		r = 'i'
	case 'ȷ':
		r = 'j'
	}
	if composed, ok := compositions[[2]rune{r, mark}]; ok {
		b.WriteRune(composed)
	} else {
		b.WriteRune(r)
		b.WriteRune(mark)
	}
	b.WriteString(base[size:])
}

func isLetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }
