package gocc

import (
	"os"
	"testing"
)

type benchInput struct {
	name, conv string
	in         []byte
}

// benchInputs are the golden text in Simplified (s2*) and Traditional
// (t2s), and a mostly ASCII text for the unmatched path.
func benchInputs(tb testing.TB) []benchInput {
	tb.Helper()
	zhs, err := os.ReadFile("testdata/golden/us_constitution_zhs.txt")
	if err != nil {
		tb.Fatal(err)
	}
	zht, err := os.ReadFile("testdata/golden/us_constitution_zhs.s2t.txt")
	if err != nil {
		tb.Fatal(err)
	}
	ascii, err := os.ReadFile("README.md")
	if err != nil {
		tb.Fatal(err)
	}
	return []benchInput{
		{"s2t", "s2t", zhs},
		{"s2twp", "s2twp", zhs},
		{"t2s", "t2s", zht},
		{"ascii", "s2t", ascii},
	}
}

func BenchmarkAppendConvert(b *testing.B) {
	for _, bi := range benchInputs(b) {
		b.Run(bi.name, func(b *testing.B) {
			c := MustNew(bi.conv)
			dst := c.AppendConvert(nil, bi.in)
			b.SetBytes(int64(len(bi.in)))
			b.ReportAllocs()
			for b.Loop() {
				dst = c.AppendConvert(dst[:0], bi.in)
			}
		})
	}
}

// BenchmarkConvertString is the string API; example_test.go has
// BenchmarkConvert.
func BenchmarkConvertString(b *testing.B) {
	for _, bi := range benchInputs(b) {
		b.Run(bi.name, func(b *testing.B) {
			c, in := MustNew(bi.conv), string(bi.in)
			b.SetBytes(int64(len(in)))
			b.ReportAllocs()
			for b.Loop() {
				c.Convert(in)
			}
		})
	}
}

// BenchmarkBuild compiles a config with a cold dictionary cache.
func BenchmarkBuild(b *testing.B) {
	dicts := embeddedDicts()
	if dicts == nil {
		b.Skip("built without dictionaries")
	}
	data, err := configFS.ReadFile("data/config/s2twp.json")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := newLoader(nil, dicts).build(data, false); err != nil {
			b.Fatal(err)
		}
	}
}

// func BenchmarkIsZhS(b *testing.B) {
// 	text := string(benchInputs(b)[0].in)
// 	b.SetBytes(int64(len(text)))
// 	b.ReportAllocs()
// 	for b.Loop() {
// 		IsZhS(text)
// 	}
// }
