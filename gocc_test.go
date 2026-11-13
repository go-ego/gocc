package gocc

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
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
