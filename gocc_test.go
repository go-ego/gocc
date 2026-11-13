package gocc

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"unicode/utf8"
	"unsafe"

	"github.com/go-ego/gocc/internal/dictfile"
)

// trailingComma matches the trailing commas upstream testcases.json allows.
var trailingComma = regexp.MustCompile(`,(\s*[}\]])`)

// TestOpenCCCases runs OpenCC's test/testcases/testcases.json.
func TestOpenCCCases(t *testing.T) {
	data, err := os.ReadFile("testdata/testcases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			ID       string            `json:"id"`
			Input    string            `json:"input"`
			Expected map[string]string `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(trailingComma.ReplaceAll(data, []byte("$1")), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) < 200 {
		t.Fatalf("only %d cases parsed", len(doc.Cases))
	}

	convs := make(map[string]*Converter)
	n := 0
	for _, c := range doc.Cases {
		for name, want := range c.Expected {
			conv, ok := convs[name]
			if !ok {
				// upstream runs these cases with --include-tofu-risk-dictionaries
				conv, err = New(name, WithTofuRisk())
				if err != nil {
					t.Fatal(err)
				}
				convs[name] = conv
			}
			if got := conv.Convert(c.Input); got != want {
				t.Errorf("%s %s(%q)\n got: %q\nwant: %q", c.ID, name, c.Input, got, want)
			}
			n++
		}
	}
	t.Logf("%d checks over %d conversions", n, len(convs))
}

// TestGolden converts the US constitution like OpenCC's golden test.
func TestGolden(t *testing.T) {
	in, err := os.ReadFile("testdata/golden/us_constitution_zhs.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"s2t", "s2tw", "s2twp", "s2hk", "s2hkp"} {
		want, err := os.ReadFile("testdata/golden/us_constitution_zhs." + name + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		got := MustNew(name).Convert(string(in))
		if got != string(want) {
			gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
			for i := 0; i < len(gl) && i < len(wl); i++ {
				if gl[i] != wl[i] {
					t.Errorf("%s line %d\n got: %s\nwant: %s", name, i+1, gl[i], wl[i])
					break
				}
			}
			if len(gl) != len(wl) {
				t.Errorf("%s: %d lines, want %d", name, len(gl), len(wl))
			}
		}
	}
}

func TestConversions(t *testing.T) {
	names := Conversions()
	want := []string{"hk2s", "hk2sp", "hk2t", "jp2t", "s2hk", "s2hkp", "s2seal", "s2t",
		"s2tw", "s2twp", "seal2t", "t2hk", "t2jp", "t2s", "t2seal", "t2tw", "tw2s", "tw2sp", "tw2t"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", names)
	}
	for _, name := range names {
		c, err := New(name + ".json")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if c.Name() == "" || c.Convert("") != "" || c.Convert("abc 123") != "abc 123" {
			t.Errorf("%s: name %q or passthrough broken", name, c.Name())
		}
	}
}

func TestNewErrors(t *testing.T) {
	for _, name := range []string{"", "x2y", "../config/s2t", "config/s2t"} {
		if _, err := New(name); err == nil {
			t.Errorf("New(%q) want error", name)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("MustNew want panic")
		}
	}()
	MustNew("x2y")
}

func TestConvert(t *testing.T) {
	cases := []struct{ conv, in, want string }{
		{"s2t", "不能踏入同一条河流", "不能踏入同一條河流"},
		{"s2t", "头发和发财", "頭髮和發財"},
		{"s2twp", "鼠标和内存", "滑鼠和記憶體"},
		{"tw2sp", "滑鼠和記憶體", "鼠标和内存"},
		{"t2s", "香煙（英語：Cigarette）", "香烟（英语：Cigarette）"},
		// invalid UTF-8 and mixed scripts pass through
		{"s2t", "a\xffb汉", "a\xffb漢"},
		// an ideographic description sequence is kept whole
		{"s2t", "⿰氵发发", "⿰氵发發"},
		// an incomplete sequence is converted rune by rune
		{"s2t", "⿰发", "⿰發"},
		// CJK compatibility ideographs are normalized first
		{"t2s", "\uF900", "岂"},
	}
	for _, c := range cases {
		if got := MustNew(c.conv).Convert(c.in); got != c.want {
			t.Errorf("%s(%q) = %q, want %q", c.conv, c.in, got, c.want)
		}
	}
}

func TestTofuRisk(t *testing.T) {
	plain, tofu := MustNew("t2s"), MustNew("t2s", WithTofuRisk())
	if got := plain.Convert("㑮"); got != "㑮" {
		t.Errorf("plain t2s(㑮) = %q", got)
	}
	if got := tofu.Convert("㑮"); got != "𫝈" {
		t.Errorf("tofu t2s(㑮) = %q", got)
	}
}

func TestAppendConvert(t *testing.T) {
	c := MustNew("s2t")
	got := c.AppendConvert([]byte(">"), []byte("汉字"))
	if string(got) != ">漢字" {
		t.Fatalf("got %q", got)
	}
}

func TestConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := MustNew("s2twp")
			for range 50 {
				if got := c.Convert("鼠标和内存"); got != "滑鼠和記憶體" {
					t.Errorf("got %q", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestSharedDicts(t *testing.T) {
	a, b := MustNew("s2t"), MustNew("s2t")
	if a.chain[0].(*shortCircuit).ms[0] != b.chain[0].(*shortCircuit).ms[0] {
		t.Fatal("converters of one config should share tries")
	}
}

func testConfig(t *testing.T, config string, dicts fstest.MapFS, opts ...Option) *Converter {
	t.Helper()
	c, err := NewFromConfig([]byte(config), dicts, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestConfigFile ports OpenCC's ConfigTest with test/config_test.
func TestConfigFile(t *testing.T) {
	dicts := os.DirFS("testdata/config_test")
	in, want := "燕燕于飞差池其羽之子于归远送于野", "燕燕于飛差池其羽之子于歸遠送於野"
	for _, file := range []string{"config_test.json", "config_test_nested_group.json"} {
		data, err := os.ReadFile("testdata/config_test/" + file)
		if err != nil {
			t.Fatal(err)
		}
		c, err := NewFromConfig(data, dicts)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Convert(in); got != want {
			t.Errorf("%s: got %q, want %q", file, got, want)
		}
	}
}

func TestInlineConfig(t *testing.T) {
	chain := testConfig(t, `{"name":"x",
		"segmentation":{"type":"mmseg","dict":{"type":"inline","entries":{"A":"A","B":"B"}}},
		"conversion_chain":[{"dict":{"type":"inline","entries":{"A":"B"}}},
			{"dict":{"type":"inline","entries":{"B":"C"}}}]}`, nil)
	if got := chain.Convert("A"); got != "C" {
		t.Errorf("chain: got %q", got)
	}

	seg := testConfig(t, `{"name":"x",
		"segmentation":{"type":"mmseg","dict":{"type":"inline","entries":{"ABC":"ABC","AB":"AB","D":"D"}}},
		"conversion_chain":[{"dict":{"type":"inline","entries":{"ABC":"X","AB":"Y","C":"Z","D":"D"}}}]}`, nil)
	if got := seg.Convert("ABCD"); got != "XD" {
		t.Errorf("segmentation: got %q", got)
	}

	exact := testConfig(t, `{"name":"x",
		"conversion_chain":[{"dict":{"type":"inline","entries":{" A ":"B"}}}]}`, nil)
	if got := exact.Convert("A"); got != "A" {
		t.Errorf("exact: got %q", got)
	}
	if got := exact.Convert(" A "); got != "B" {
		t.Errorf("exact: got %q", got)
	}
}

// TestSegmentationBoundary checks that the chain never matches across
// segment boundaries.
func TestSegmentationBoundary(t *testing.T) {
	dicts := fstest.MapFS{
		"seg.txt":   {Data: []byte("AB\tAB\n")},
		"conv.txt":  {Data: []byte("BC\tX\n")},
		"conv2.txt": {Data: []byte("C\tc\n")},
	}
	c := testConfig(t, `{"name":"x",
		"segmentation":{"type":"mmseg","dict":{"type":"text","file":"seg.txt"}},
		"conversion_chain":[{"dict":{"type":"group","dicts":[
			{"type":"text","file":"conv.txt"},{"type":"text","file":"conv2.txt"}]}}]}`, dicts)
	if got := c.Convert("ABC"); got != "ABc" {
		t.Errorf("got %q, want ABc", got)
	}
}

func TestMatchPolicy(t *testing.T) {
	dicts := fstest.MapFS{
		"a.ocd2": {Data: []byte("not a text dict")},
		"a.txt":  {Data: []byte("A\t1\nAB\t2\n")},
		"b.txt":  {Data: []byte("ABC\t3\nAB\t4\n")},
		"c.txt":  {Data: []byte("ABCD\t5\n")},
	}
	cases := []struct{ name, group, want string }{
		{"short_circuit", `{"type":"group","match_policy":"short_circuit","dicts":[
			{"type":"ocd2","file":"a.ocd2"},{"type":"text","file":"b.txt"}]}`, "2C"},
		{"default", `{"type":"group","dicts":[
			{"type":"text","file":"a.txt"},{"type":"text","file":"b.txt"}]}`, "2C"},
		{"union", `{"type":"group","match_policy":"union","dicts":[
			{"type":"text","file":"a.txt"},{"type":"text","file":"b.txt"}]}`, "3"},
		{"union tie", `{"type":"group","match_policy":"union","dicts":[
			{"type":"text","file":"b.txt"},{"type":"text","file":"a.txt"}]}`, "3"},
		{"union of groups", `{"type":"group","match_policy":"union","dicts":[
			{"type":"group","match_policy":"short_circuit","dicts":[{"type":"text","file":"a.txt"}]},
			{"type":"inline","entries":{"ABC":"6"}}]}`, "6"},
		{"tofu dropped", `{"type":"group","match_policy":"union","dicts":[
			{"type":"text","file":"c.txt","may_output_tofu":true},{"type":"text","file":"b.txt"}]}`, "3"},
	}
	for _, c := range cases {
		conv := testConfig(t, `{"name":"x","conversion_chain":[{"dict":`+c.group+`}]}`, dicts)
		if got := conv.Convert("ABC"); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}

	union := `{"type":"group","match_policy":"union","dicts":[
		{"type":"text","file":"c.txt","may_output_tofu":true},{"type":"text","file":"b.txt"}]}`
	conv := testConfig(t, `{"name":"x","conversion_chain":[{"dict":`+union+`}]}`, dicts, WithTofuRisk())
	if got := conv.Convert("ABCD"); got != "5" {
		t.Errorf("tofu included: got %q", got)
	}

	// everything excluded leaves the text unchanged
	none := testConfig(t, `{"name":"x","conversion_chain":[{"dict":
		{"type":"group","dicts":[{"type":"text","file":"c.txt","may_output_tofu":true}]}}]}`, dicts)
	if got := none.Convert("ABCD"); got != "ABCD" {
		t.Errorf("none: got %q", got)
	}
}

func TestConfigErrors(t *testing.T) {
	dicts := fstest.MapFS{"bad.txt": {Data: []byte("no tab\n")}}
	bad := []string{
		`{`,
		`{"name":"x"}`,
		`{"name":"x","conversion_chain":[{}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"zip","file":"a"}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"text"}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"text","file":"missing.txt"}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"text","file":"bad.txt"}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"group","match_policy":"x","dicts":[]}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline","entries":{"a":""}}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline","may_output_tofu":true,"entries":{"a":"b"}}}]}`,
		`{"name":"x","segmentation":{"type":"jieba"},"conversion_chain":[{"dict":{"type":"inline","entries":{"a":"b"}}}]}`,
		`{"name":"x","segmentation":{"type":"mmseg"},"conversion_chain":[{"dict":{"type":"inline","entries":{"a":"b"}}}]}`,
		`{"name":"x","normalization":[{"dict":{"type":"zip"}}],"conversion_chain":[{"dict":{"type":"inline","entries":{"a":"b"}}}]}`,
	}
	for _, config := range bad {
		if _, err := NewFromConfig([]byte(config), dicts); err == nil {
			t.Errorf("NewFromConfig(%s) want error", config)
		}
	}
}

func TestIDS(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"⿰氵青x", len("⿰氵青")},
		{"⿲彳亍亍", len("⿲彳亍亍")},
		{"⿱⿰木木木", len("⿱⿰木木木")},
		{"⿾人", len("⿾人")},
		{"⿰氵", len("⿰")},     // incomplete
		{"⿰\xff青", len("⿰")}, // invalid operand
		{"汉", len("汉")},
		{"\xff", 1},
		{strings.Repeat("⿰", 20) + strings.Repeat("木", 21), len("⿰")}, // too deep
	}
	for _, c := range cases {
		b := []byte(c.in)
		r, size := runeAt(b)
		if got := skipLen(b, r, size); got != c.want {
			t.Errorf("skipLen(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestSegment(t *testing.T) {
	m, err := newTrie(nil)
	if err != nil {
		t.Fatal(err)
	}
	var segs []string
	segment(m, []byte("abc"), func(s []byte) { segs = append(segs, string(s)) })
	if len(segs) != 1 || segs[0] != "abc" {
		t.Fatalf("got %q", segs)
	}
}

func TestTrieLongest(t *testing.T) {
	m := testConfig(t, `{"name":"x","conversion_chain":[{"dict":{"type":"inline",
		"entries":{"中":"1","中国":"2","中国人民":"3"}}}]}`, nil)
	cases := map[string]string{"中国人": "2人", "中国人民": "3", "中中": "11", "国": "国"}
	keys := make([]string, 0, len(cases))
	for k := range cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, in := range keys {
		if got := m.Convert(in); got != cases[in] {
			t.Errorf("%q: got %q, want %q", in, got, cases[in])
		}
	}
}

// TestNUL guards against cedar's terminal edge (label 0) swallowing a NUL
// byte that follows a key.
func TestNUL(t *testing.T) {
	m := testConfig(t, `{"name":"x","conversion_chain":[{"dict":{"type":"inline",
		"entries":{"a":"1","ab":"2"}}}]}`, nil)
	cases := map[string]string{"a\x00b": "1\x00b", "\x00": "\x00", "a\x00": "1\x00", "\x00ab": "\x002"}
	for in, want := range cases {
		if got := m.Convert(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	if got := MustNew("s2t").Convert("干\x00x头"); got != "幹\x00x頭" {
		t.Errorf("s2t: got %q", got)
	}
	if _, err := NewFromConfig([]byte(`{"name":"x","conversion_chain":[{"dict":{"type":"inline",
		"entries":{"a\u0000":"1"}}}]}`), nil); err == nil {
		t.Error("want error for NUL key")
	}
}

func TestConfigUpstreamCompat(t *testing.T) {
	// comments and trailing commas, as OpenCC accepts
	c := testConfig(t, `{
		// line comment
		"name": "x", /* block */
		"conversion_chain": [{"dict": {"type": "inline", "entries": {"a": "b//c", "d": "/*e*/",},},},],
	}`, nil)
	if got := c.Convert("ad"); got != "b//c/*e*/" {
		t.Errorf("jsonc: got %q", got)
	}
	// an empty chain is a no-op
	if got := testConfig(t, `{"name":"x","conversion_chain":[]}`, nil).Convert("ab"); got != "ab" {
		t.Errorf("empty chain: got %q", got)
	}
	// inline entries keep document order: segmentation and chain are stable
	for range 20 {
		m := testConfig(t, `{"name":"x","conversion_chain":[{"dict":{"type":"inline",
			"entries":{"中":"Z","中国":"ZG"}}}]}`, nil)
		if got := m.Convert("中国中"); got != "ZGZ" {
			t.Fatalf("got %q", got)
		}
	}
}

func TestConfigStrictErrors(t *testing.T) {
	bad := []string{
		`{"name":"x","conversion_chain":[{"dict":{"type":"group"}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"group","match_policy":"","dicts":[{"type":"inline","entries":{"a":"b"}}]}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline"}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline","entries":{"a":"1","a":"2"}}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline","may_output_tofu":false,"entries":{"a":"b"}}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline","entries":[]}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline","entries":{"a":1}}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"inline","entries":{"\xff":"b"}}}]}`,
		`{"name":"x","conversion_chain":[{"dict":{"type":"text","file":"a.txt"}}]}`, // nil FS
		`{"name":"x" /* unterminated`,
	}
	for _, config := range bad {
		if _, err := NewFromConfig([]byte(config), nil); err == nil {
			t.Errorf("NewFromConfig(%s) want error", config)
		}
	}
}

// TestConfigTofuValidation checks that, like Config.cpp, a dictionary
// dropped for its tofu risk is not validated further, an inline one is
// always rejected, and the group policy does not change either rule.
func TestConfigTofuValidation(t *testing.T) {
	cases := []struct {
		dict string
		ok   bool // without WithTofuRisk; with it every case is an error
	}{
		{`{"type":"zip","may_output_tofu":true}`, true},
		{`{"type":"text","may_output_tofu":true}`, true},
		{`{"type":"group","may_output_tofu":true}`, true},
		{`{"type":"group","match_policy":"union","may_output_tofu":true}`, true},
		{`{"type":"inline","may_output_tofu":true,"entries":{"a":"b"}}`, false},
		{`{"type":"group","match_policy":"union"}`, false},
	}
	for _, c := range cases {
		for _, policy := range []string{"short_circuit", "union"} {
			config := `{"name":"x","conversion_chain":[{"dict":{"type":"group","match_policy":"` +
				policy + `","dicts":[` + c.dict + `]}}]}`
			conv, err := NewFromConfig([]byte(config), nil)
			if (err == nil) != c.ok {
				t.Errorf("%s in %s: err = %v, want ok %v", c.dict, policy, err, c.ok)
			} else if c.ok && conv.Convert("a") != "a" {
				t.Errorf("%s in %s: dropped dict converts", c.dict, policy)
			}
			if _, err := NewFromConfig([]byte(config), nil, WithTofuRisk()); err == nil {
				t.Errorf("%s in %s with tofu risk: want error", c.dict, policy)
			}
		}
	}
}

// TestConfigCommas checks only a comma after a value may trail.
func TestConfigCommas(t *testing.T) {
	entries := func(body string) string {
		return `{"name":"x","conversion_chain":[{"dict":{"type":"inline","entries":` + body + `}}]}`
	}
	if got := testConfig(t, entries(`{"a":"b", /* c */ }`), nil).Convert("a"); got != "b" {
		t.Errorf("trailing comma: got %q", got)
	}
	bad := []string{
		`{"name":"x","conversion_chain":[,]}`,
		`{"name":"x","conversion_chain":[],,}`,
		`{,"name":"x","conversion_chain":[]}`,
		entries(`{,}`),
		entries(`{"a":,}`),
		entries(`{"a":"b",,}`),
	}
	for _, config := range bad {
		if _, err := NewFromConfig([]byte(config), nil); err == nil {
			t.Errorf("NewFromConfig(%s) want error", config)
		}
	}
}

// TestOcdAliasShared checks X.ocd2 and X.txt resolve to one cached trie.
func TestOcdAliasShared(t *testing.T) {
	l := newLoader(nil, fstest.MapFS{"a.txt": {Data: []byte("a\tb\n")}})
	x, err := l.dict(&dictSpec{Type: "ocd2", File: "a.ocd2"}, false)
	if err != nil {
		t.Fatal(err)
	}
	y, err := l.dict(&dictSpec{Type: "text", File: "a.txt"}, false)
	if err != nil || x != y {
		t.Fatalf("not shared: %v", err)
	}
}

// TestInvalidUTF8 checks keys still match before stray bytes.
func TestInvalidUTF8(t *testing.T) {
	c := MustNew("s2t")
	cases := map[string]string{"发\x80": "發\x80", "头发\x80": "頭髮\x80", "汉\xff": "漢\xff", "\xe6\xb1": "\xe6\xb1"}
	for in, want := range cases {
		if got := c.Convert(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	m := testConfig(t, `{"name":"x","normalization":[{"dict":{"type":"inline","entries":{"\uF900":"豈"}}}],
		"conversion_chain":[{"dict":{"type":"inline","entries":{"豈":"岂"}}}]}`, nil)
	for in, want := range map[string]string{"a\uF900b": "a岂b", "ab": "ab", "豈": "岂", "": ""} {
		if got := m.Convert(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	in := []byte("abc")
	n, _ := m.norm[0].(*trie), 0
	if out := normalize(n, in); &out[0] != &in[0] {
		t.Error("unchanged text should not be copied")
	}
}

// TestTrieDuplicates checks the earliest list wins a duplicate key, and
// that a key is not taken for present when it only prefixes another.
func TestTrieDuplicates(t *testing.T) {
	m, err := newTrie(
		[]dictfile.Entry{{Key: "ab", Values: []string{"1"}}},
		[]dictfile.Entry{{Key: "a", Values: []string{"2"}}, {Key: "ab", Values: []string{"3"}},
			{Key: "abc", Values: []string{"4"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{"a": "2", "ab": "1", "abc": "4", "abab": "11", "b": "b"} {
		if got := string(convert(nil, m, []byte(in))); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// TestRuneAt checks the decoder against unicode/utf8 on every three-byte
// sequence, which it decodes itself, and on the other lengths.
func TestRuneAt(t *testing.T) {
	check := func(s []byte) {
		r, size := runeAt(s)
		if wr, wsize := utf8.DecodeRune(s); r != wr || size != wsize {
			t.Fatalf("runeAt(% x) = %U, %d; want %U, %d", s, r, size, wr, wsize)
		}
	}
	for b0 := 0xE0; b0 <= 0xEF; b0++ {
		for b1 := range 0x100 {
			for b2 := range 0x100 {
				check([]byte{byte(b0), byte(b1), byte(b2)})
			}
		}
	}
	for _, s := range []string{"a", "\x00", "é", "\xe4", "\xe4\xb8", "😀", "\U0002F800", "\xf0\x9f\x98", "\x80", "\xff"} {
		check([]byte(s))
	}
}

// TestConvertReference compares s2t's matcher with a map-based longest
// prefix match on random mixes of keys, cut keys, NUL and invalid bytes.
func TestConvertReference(t *testing.T) {
	read := func(name string) []dictfile.Entry {
		data, err := os.ReadFile("data/dictionary/" + name)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := dictfile.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		return entries
	}
	// s2t: short_circuit[union[STPhrases, generated], STCharacters]
	phrases, chars := map[string]string{}, map[string]string{}
	var keys []string
	maxLen := 0
	for _, e := range append(read("STPhrases.txt"), read("STPhrases_GeneratedFromRegionalPhrases.txt")...) {
		if _, ok := phrases[e.Key]; !ok {
			phrases[e.Key] = e.Values[0]
			keys = append(keys, e.Key)
			maxLen = max(maxLen, len(e.Key))
		}
	}
	for _, e := range read("STCharacters.txt") {
		chars[e.Key] = e.Values[0]
		keys = append(keys, e.Key)
	}
	reference := func(s string) string {
		var out []byte
		for i := 0; i < len(s); {
			n, val := 0, ""
			for _, dict := range []map[string]string{phrases, chars} {
				for j := min(len(s), i+maxLen); j > i && n == 0; j-- {
					if v, ok := dict[s[i:j]]; ok {
						n, val = j-i, v
					}
				}
			}
			if n == 0 {
				_, n = utf8.DecodeRuneInString(s[i:])
				val = s[i : i+n]
			}
			out = append(out, val...)
			i += n
		}
		return string(out)
	}

	noise := []string{"a", " ", "，", "\x00", "\xff", "\xe6\xb1", "\uFFFD", "𫝈", "鿿", "\n"}
	rng := rand.New(rand.NewPCG(1, 2))
	m := MustNew("s2t").chain[0]
	for range 3000 {
		var in []byte
		for range 1 + rng.IntN(12) {
			switch key := keys[rng.IntN(len(keys))]; rng.IntN(4) {
			case 0:
				in = append(in, noise[rng.IntN(len(noise))]...)
			case 1:
				in = append(in, key[:rng.IntN(len(key))]...) // may cut a rune
			default:
				in = append(in, key...)
			}
		}
		if got, want := string(convert(nil, m, in)), reference(string(in)); got != want {
			t.Fatalf("convert(%q)\n got: %q\nwant: %q", in, got, want)
		}
	}
}

// TestAllocs guards the allocation-free conversion path.
func TestAllocs(t *testing.T) {
	in := "鼠标和内存 hello 头发发财，不能踏入同一条河流"
	for _, name := range []string{"s2t", "t2s"} { // single conversions use no pool
		c := MustNew(name)
		src, dst := []byte(in), make([]byte, 0, 4*len(in))
		if n := testing.AllocsPerRun(100, func() { dst = c.AppendConvert(dst[:0], src) }); n != 0 {
			t.Errorf("%s: AppendConvert allocates %v times", name, n)
		}
		if n := testing.AllocsPerRun(100, func() { c.Convert(in) }); n != 1 {
			t.Errorf("%s: Convert allocates %v times, want 1", name, n)
		}
	}
}

// TestConvertString checks Convert neither aliases nor changes its input.
func TestConvertString(t *testing.T) {
	c := MustNew("s2t")
	in := strings.Repeat("abc", 3)
	out := c.Convert(in)
	if out != in || unsafe.StringData(out) == unsafe.StringData(in) {
		t.Fatalf("unchanged text: got %q, aliased %v", out, unsafe.StringData(out) == unsafe.StringData(in))
	}
	in = strings.Repeat("汉字", 3)
	if got := c.Convert(in); got != "漢字漢字漢字" || in != "汉字汉字汉字" {
		t.Fatalf("got %q, input now %q", got, in)
	}
}
