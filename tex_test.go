package bibtex_test

import (
	"testing"

	"github.com/git-pkgs/bibtex"
)

func TestPlainText(t *testing.T) {
	tests := []struct{ in, want string }{
		{`M{\"u}ller`, muller},
		{`M\"uller`, muller},
		{`Dvo{\v{r}}{\'a}k`, "Dvořák"},
		{`Dvo\v rák`, "Dvořák"},
		{`Erd{\H{o}}s`, "Erdős"},
		{`{\k{a}}`, "ą"},
		{`Do{\u{g}}an`, "Doğan"},
		{`Fran{\c{c}}ois`, "François"},
		{`{\r{A}}ngstr{\"o}m`, "Ångström"},
		{`{\=a}`, "ā"},
		{`{\.z}`, "ż"},
		{`{\d{s}}`, "ṣ"},
		{`{\b{t}}`, "ṯ"},
		{`{\'\i}`, "í"},
		{`{\'{\i}}`, "í"},
		{`{\"\i}`, "ï"},
		{`\'{\"u}`, "ǘ"},
		{`{\t{oo}}`, "o͡o"},
		{`{\v{q}}`, "q̌"},
		{`{\o}ster {\O}`, "øster Ø"},
		{`{\ss} {\aa} {\AA} {\ae} {\OE} {\l}{\L}`, "ß å Å æ Œ łŁ"},
		{`Stra\ss e`, "Straße"},
		{`{\L}{\'o}d{\'z}`, "Łódź"},
		{`\LaTeX{} and \TeX\ and \BibTeX`, "LaTeX and TeX and BibTeX"},
		{`A {GPU} {L}ibrary`, "A GPU Library"},
		{`\textbf{Bold} \emph{em} \textit{it}`, "Bold em it"},
		{`\url{https://example.org/a~b--c}`, "https://example.org/a~b--c"},
		{`\href{https://example.org}{Example}`, "Example"},
		{`pages 1--10, a---b`, "pages 1–10, a—b"},
		{"``quoted''", "“quoted”"},
		{`O'Brien`, "O'Brien"},
		{`Smith~J.`, "Smith J."},
		{`R\&D 50\% \$5 \#1 a\_b \{x\}`, "R&D 50% $5 #1 a_b {x}"},
		{`$O(n^2)$ sort`, "$O(n^2)$ sort"},
		{`$\alpha$-helix`, "$\\alpha$-helix"},
		{`unclosed $math`, "unclosed $math"},
		{`J.\,R.\,R. Tolkien`, "J. R. R. Tolkien"},
		{`co\-operation`, "cooperation"},
		{`\unknown{kept} text`, "kept text"},
		{`\copyright{} \texttrademark`, "© ™"},
		{"  spaced \n  out  ", "spaced out"},
		{`Ünïcode — kept`, "Ünïcode — kept"},
		{`trailing \`, "trailing"},
		{`\'`, ""},
		{`\'{}x`, "x"},
		{`{{{\'e}}}`, "é"},
	}
	for _, tc := range tests {
		if got := bibtex.PlainText(tc.in); got != tc.want {
			t.Errorf("PlainText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
