package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("不能踏入同一条河流\n鼠标\n")
	if err := run([]string{"-c", "s2twp"}, in, &out); err != nil {
		t.Fatal(err)
	}
	if want := "不能踏入同一條河流\n滑鼠\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestRunFiles(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.txt"), filepath.Join(dir, "out.txt")
	if err := os.WriteFile(in, []byte("漢字"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-c", "t2s", "-i", in, "-o", out}, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "汉字" {
		t.Fatalf("got %q", got)
	}
}

// TestRunSameFile guards against truncating the input before reading it.
func TestRunSameFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(f, []byte("汉字"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-i", f, "-o", f}, nil, nil); err == nil {
		t.Fatal("want same-file error")
	}
	if got, err := os.ReadFile(f); err != nil || string(got) != "汉字" {
		t.Fatalf("input damaged: %q %v", got, err)
	}
}

func TestRunConfigFile(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "my.json")
	err := os.WriteFile(config, []byte(`{"name":"my","conversion_chain":[{"dict":{"type":"group","dicts":[
		{"type":"text","file":"my.txt"},{"type":"ocd2","file":"STCharacters.ocd2"}]}}]}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "my.txt"), []byte("内存\t記憶體\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"-c", config}, strings.NewReader("内存汉字"), &out); err != nil {
		t.Fatal(err)
	}
	if want := "記憶體漢字"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestRunDictPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "STPhrases.txt"), []byte("内存\t記憶體\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"-c", "s2t", "-path", dir}, strings.NewReader("内存汉字"), &out); err != nil {
		t.Fatal(err)
	}
	if want := "記憶體漢字"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestRunErrors(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-c", "nope"}, strings.NewReader(""), &out); err == nil {
		t.Fatal("want unknown conversion error")
	}
	if err := run([]string{"-i", "/nonexistent/x"}, nil, &out); err == nil {
		t.Fatal("want missing input error")
	}
}
