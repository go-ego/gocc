package gocc

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, name, data string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewFromFile(t *testing.T) {
	c, err := NewFromFile("testdata/config_test/config_test.json")
	if err != nil {
		t.Fatal(err)
	}
	in, want := "燕燕于飞差池其羽之子于归远送于野", "燕燕于飛差池其羽之子于歸遠送於野"
	if got := c.Convert(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestNewFromFileBuiltin checks dictionaries missing from the config's
// directory come from the built-in ones, sharing their tries.
func TestNewFromFileBuiltin(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "my.json")
	writeFile(t, config, `{"name":"my","conversion_chain":[
		{"dict":{"type":"group","dicts":[
			{"type":"text","file":"my.txt"},
			{"type":"ocd2","file":"STCharacters.ocd2"}]}}]}`)
	writeFile(t, filepath.Join(dir, "my.txt"), "内存\t記憶體\n")

	c, err := NewFromFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Convert("内存汉字"), "記憶體漢字"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	chars, err := builtin().trie([]string{"STCharacters.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if c.chain[0].(*shortCircuit).ms[1] != chars {
		t.Fatal("built-in dictionary should be shared")
	}
}

func TestLoadDict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "STPhrases.txt"), "内存\t記憶體\n")

	c, err := New("s2t", LoadDict(dir))
	if err != nil {
		t.Fatal(err)
	}
	// STPhrases.txt is replaced; the other dictionaries stay built-in
	if got, want := c.Convert("内存汉字"), "記憶體漢字"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := MustNew("s2t").Convert("内存"), "內存"; got != want {
		t.Fatalf("built-in converter changed: got %q, want %q", got, want)
	}

	// NewFromFile searches the config's directory before LoadDict
	cfgDir := t.TempDir()
	config := filepath.Join(cfgDir, "c.json")
	writeFile(t, config, `{"name":"c","conversion_chain":[{"dict":{"type":"text","file":"STPhrases.txt"}}]}`)
	if c, err = NewFromFile(config, LoadDict(dir)); err != nil {
		t.Fatal(err)
	}
	if got := c.Convert("内存"); got != "記憶體" {
		t.Fatalf("LoadDict: got %q", got)
	}
	writeFile(t, filepath.Join(cfgDir, "STPhrases.txt"), "内存\t內存儲器\n")
	if c, err = NewFromFile(config, LoadDict(dir)); err != nil {
		t.Fatal(err)
	}
	if got := c.Convert("内存"); got != "內存儲器" {
		t.Fatalf("config directory: got %q", got)
	}
}

func TestNewFromFileErrors(t *testing.T) {
	if _, err := NewFromFile("testdata/nonexistent.json"); err == nil {
		t.Fatal("want missing config error")
	}
	config := filepath.Join(t.TempDir(), "c.json")
	writeFile(t, config, `{"name":"c","conversion_chain":[{"dict":{"type":"text","file":"missing.txt"}}]}`)
	if _, err := NewFromFile(config); err == nil {
		t.Fatal("want missing dictionary error")
	}
	if _, err := NewFromConfig([]byte(`{"name":"c","conversion_chain":[{"dict":{"type":"text","file":"STPhrases.txt"}}]}`), nil); err == nil {
		t.Fatal("NewFromConfig must not fall back to built-in dictionaries")
	}
}

func TestTarget(t *testing.T) {
	want := map[string]string{
		"t2s": "s", "tw2s": "s", "tw2sp": "s", "hk2s": "s", "hk2sp": "s",
		"s2t": "t", "s2tw": "t", "s2twp": "t", "s2hk": "t", "s2hkp": "t",
		"t2tw": "t", "t2hk": "t", "tw2t": "t", "hk2t": "t", "seal2t": "t",
		"t2jp": "jp", "jp2t": "jp",
		"s2seal": "", "t2seal": "",
	}
	for _, name := range Conversions() {
		w, ok := want[name]
		if !ok {
			t.Errorf("%s: missing from the table", name)
			continue
		}
		c := MustNew(name)
		if c.Simplified() != (w == "s") || c.Traditional() != (w == "t") || c.JP() != (w == "jp") {
			t.Errorf("%s: Simplified %v, Traditional %v, JP %v; want %q",
				name, c.Simplified(), c.Traditional(), c.JP(), w)
		}
	}
	if c := (&Converter{name: "Configuration Test"}); c.Simplified() || c.Traditional() || c.JP() {
		t.Error("a name without a target matches")
	}
}
