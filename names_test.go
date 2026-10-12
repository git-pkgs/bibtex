package bibtex_test

import (
	"reflect"
	"testing"

	"github.com/git-pkgs/bibtex"
)

// Expected splits for the BibTeX forms match pybtex's Person parsing.
func TestParseNames(t *testing.T) {
	type n = bibtex.Name
	tests := []struct {
		in   string
		want []n
	}{
		{"Jane Doe", []n{{First: jane, Last: doe}}},
		{"Doe, Jane", []n{{First: jane, Last: doe}}},
		{doe, []n{{Last: doe}}},
		{"Ludwig van Beethoven", []n{{First: "Ludwig", Von: "van", Last: "Beethoven"}}},
		{"van Beethoven, Ludwig", []n{{First: "Ludwig", Von: "van", Last: "Beethoven"}}},
		{"Jean de la Fontaine", []n{{First: "Jean", Von: deLa, Last: "Fontaine"}}},
		{"de la Fontaine, Jean", []n{{First: "Jean", Von: deLa, Last: "Fontaine"}}},
		{"Van Rossum, Guido", []n{{First: "Guido", Last: "Van Rossum"}}},
		{"Guido Van Rossum", []n{{First: "Guido Van", Last: "Rossum"}}},
		{"Ford, Jr., Henry", []n{{First: "Henry", Last: "Ford", Jr: jrDot}}},
		{"van der Walt, Jr, Stéfan", []n{{First: "Stéfan", Von: vanDer, Last: "Walt", Jr: "Jr"}}},
		{"Charles Louis Xavier Joseph de la Vall{\\'e}e Poussin", []n{{First: "Charles Louis Xavier Joseph", Von: deLa, Last: "Vallée Poussin"}}},
		{"{\\'E}mile Zola", []n{{First: "Émile", Last: "Zola"}}},
		{"{\\'e}mile Zola", []n{{Von: "émile", Last: "Zola"}}},
		{"Kele\\c{s}temur, Tar{\\i}k", []n{{First: "Tarık", Last: "Keleştemur"}}},
		{"{World Health Organization}", []n{{Last: "World Health Organization", Literal: true}}},
		{"{Barnes and Noble}", []n{{Last: "Barnes and Noble", Literal: true}}},
		{`{\"O}zil`, []n{{Last: "Özil"}}},
		{"{Barnes} {Noble}", []n{{First: "Barnes", Last: "Noble"}}},
		{"{van} Gogh, Vincent", []n{{First: "Vincent", Last: "van Gogh"}}},
		{"Jean-Paul Sartre", []n{{First: "Jean-Paul", Last: "Sartre"}}},
		{"Ünal, Özge", []n{{First: "Özge", Last: "Ünal"}}},
		{"Doe, Jane and Smith, John AND Roe, R.", []n{{First: jane, Last: doe}, {First: "John", Last: "Smith"}, {First: "R.", Last: "Roe"}}},
		{"Doe, Jane and others", []n{{First: jane, Last: doe}, {Others: true}}},
		// BibTeX collapses whitespace before splitting; pybtex does not.
		{"Doe,\n  Jane\n  and\tSmith, John", []n{{First: jane, Last: doe}, {First: "John", Last: "Smith"}}},
		{"Sandersand, A and Anders, B", []n{{First: "A", Last: "Sandersand"}, {First: "B", Last: "Anders"}}},
		{"林知樹 and 小林和弘", []n{{Last: "林知樹"}, {Last: "小林和弘"}}},
		{"", nil},
		{" and ", nil},
	}
	for _, tc := range tests {
		if got := bibtex.ParseNames(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseNames(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestParseNamesCommaList(t *testing.T) {
	type n = bibtex.Name
	tests := []struct {
		in   string
		want []n
	}{
		{"Jifeng Dai, Yi Li, Kaiming He", []n{{First: "Jifeng", Last: "Dai"}, {First: "Yi", Last: "Li"}, {First: "Kaiming", Last: "He"}}},
		{"Minguk Kang, Woohyeon Shim, and Jaesik Park", []n{{First: "Minguk", Last: "Kang"}, {First: "Woohyeon", Last: "Shim"}, {First: "Jaesik", Last: "Park"}}},
		// One-word parts keep BibTeX's "von Last, Jr, First" reading.
		{"Ford, Jr., Henry", []n{{First: "Henry", Last: "Ford", Jr: jrDot}}},
		// A single comma is BibTeX's "Last, First" even with several words.
		{"Lei Chen, Le Wu", []n{{First: "Le Wu", Last: "Lei Chen"}}},
	}
	for _, tc := range tests {
		if got := bibtex.ParseNames(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseNames(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestNameString(t *testing.T) {
	for _, tc := range []struct {
		n    bibtex.Name
		want string
	}{
		{bibtex.Name{First: "Jan", Von: vanDer, Last: "Berg", Jr: jrDot}, "Jan van der Berg, Jr."},
		{bibtex.Name{Last: "WHO"}, "WHO"},
		{bibtex.Name{Others: true}, "others"},
	} {
		if got := tc.n.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.n, got, tc.want)
		}
	}
}
