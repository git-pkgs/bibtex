package bibtex_test

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/git-pkgs/bibtex"
)

// Inputs here are hostile or pathological shapes a blob scan can meet. Each
// must finish quickly, without panicking, within a memory budget.
func TestPathologicalInputs(t *testing.T) {
	const size = 4 << 20
	const entries = 150_000
	tests := []struct {
		name string
		data string
		want error
	}{
		{"unterminated quote", `@string{a = "b`, nil},
		{"unterminated quoted field", `@misc{x, title = "T`, nil},
		{"at signs", strings.Repeat("@", size), nil},
		{"line-start at signs", strings.Repeat("\n@", size/2), nil},
		{"open entries", strings.Repeat("@a{", size/3), nil},
		{"open entries on lines", strings.Repeat("@a{k,\n", entries), nil},
		{"open values", strings.Repeat("@a{k, t = {\n", entries), nil},
		{"open quotes", strings.Repeat("@a{k, t = \"\n", entries), nil},
		{"open comments", strings.Repeat("@comment{", size/9), nil},
		{"one line", strings.Repeat("@misc{k, a = {b}, c = d # \"e\"} ", entries), nil},
		{"many fields", "@misc{k," + strings.Repeat("a = 1,", size/6) + "}", nil},
		{"many keys", strings.Repeat("@misc{k,}\n", entries), nil},
		{"junk fields", "@misc{k," + strings.Repeat("; ", size/2) + "}", nil},
		{"deep braces", "@misc{k, t = " + strings.Repeat("{", size), bibtex.ErrLimit},
		{"entry limit", strings.Repeat("@misc{k,}\n", 250_000), bibtex.ErrLimit},
		{"macro doubling", macroDoubling(40), bibtex.ErrLimit},
		{"macro fan-out", macroFanOut(), bibtex.ErrLimit},
		{"long value", "@misc{k, t = {" + strings.Repeat("x", 2<<20) + "}}", bibtex.ErrLimit},
		{"fences", strings.Repeat("```\n@misc{k,\n", entries), nil},
		{"indented blocks", strings.Repeat("  @misc{k, t = {\n", entries), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, extract := range []bool{false, true} {
				var err error
				allocated := allocatedBytes(func() {
					within(t, 5*time.Second, func() {
						if extract {
							_, err = bibtex.Extract([]byte(tc.data), bibtex.ParseOptions{})
						} else {
							_, err = bibtex.Parse([]byte(tc.data))
						}
					})
				})
				// Extract may skip an input before reaching the limit.
				if err != nil && !errors.Is(err, tc.want) || err == nil && tc.want != nil && !extract {
					t.Fatalf("extract=%v: %v", extract, err)
				}
				if budget := uint64(24*len(tc.data)) + 128<<20; allocated > budget {
					t.Fatalf("extract=%v: %d input bytes allocated %d", extract, len(tc.data), allocated)
				}
			}
		})
	}
}

func TestPathologicalText(t *testing.T) {
	const size = 1 << 20
	for _, s := range []string{
		strings.Repeat(`\'{`, size/3),
		strings.Repeat(`\'`, size/2),
		strings.Repeat(`\'{\'{\'{\'{\'{\'{x}}}}}}`, size/24),
		strings.Repeat("{", size),
		strings.Repeat("$", size),
		strings.Repeat(`\textbf{`, size/8),
		strings.Repeat("a and ", size/6),
		strings.Repeat("A B, ", size/5),
		strings.Repeat("{\\", size/2),
	} {
		within(t, 5*time.Second, func() {
			bibtex.PlainText(s)
			bibtex.ParseNames(s)
		})
	}
}

func macroDoubling(n int) string {
	var b strings.Builder
	b.WriteString("@string{m0 = \"xxxxxxxx\"}\n")
	for i := 1; i <= n; i++ {
		b.WriteString("@string{m" + itoa(i) + " = m" + itoa(i-1) + " # m" + itoa(i-1) + "}\n")
	}
	return b.String()
}

func macroFanOut() string {
	var b strings.Builder
	b.WriteString("@string{big = {" + strings.Repeat("x", 1<<20-16) + "}}\n")
	for i := 0; i < 100; i++ {
		b.WriteString("@misc{k" + itoa(i) + ", a = big}\n")
	}
	return b.String()
}

func itoa(i int) string {
	return strings.TrimLeft(strings.Repeat("0", 3)+string(rune('0'+i/100))+string(rune('0'+i/10%10))+string(rune('0'+i%10)), "0")
}

func within(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("did not finish within %s", d)
	}
}

func allocatedBytes(run func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}
