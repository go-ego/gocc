package dictgen

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	gocc "github.com/go-ego/gocc"
	"github.com/go-ego/gocc/internal/dictfile"
)

func TestReverse(t *testing.T) {
	data := "# @reverse-prefer: 梁\n# @reverse-prefer: 么 麼\n樑\t梁\n梁\t梁\n幺\t么\n麼\t么\n"
	got, err := Reverse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []dictfile.Entry{
		{Key: "梁", Values: []string{"梁", "樑"}},
		{Key: "么", Values: []string{"麼", "幺"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	// the last preference of a key wins
	got, err = Reverse([]byte("# @reverse-prefer: x a\n# @reverse-prefer: x c\na\tx\nb\tx\nc\tx\n"))
	if err != nil || strings.Join(got[0].Values, " ") != "c a b" {
		t.Fatalf("last preference: %+v %v", got, err)
	}

	if _, err := Reverse([]byte("# @reverse-prefer: x\na\tb\n")); err == nil {
		t.Fatal("want error for preference without mapping")
	}
}

func TestExtractTofuRisk(t *testing.T) {
	data := "# @tofu-risk: note\n㑮\t㑮 𫝈\n㑯\t㑔\n# @tofu-risk: n\n㒓\t𠉂\n"
	got, err := ExtractTofuRisk([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []dictfile.Entry{
		{Key: "㑮", Values: []string{"𫝈"}},
		{Key: "㒓", Values: []string{"𠉂"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	for _, bad := range []string{"# @tofu-risk: x\n", "# @tofu-risk: x\n# c\n", "# @tofu-risk: x\na\ta\n"} {
		if _, err := ExtractTofuRisk([]byte(bad)); err == nil {
			t.Errorf("ExtractTofuRisk(%q) want error", bad)
		}
	}
}

func TestSTPhrases(t *testing.T) {
	t2s := gocc.MustNew("t2s")
	got, err := STPhrases(t2s, []byte("記憶體\t内存\n滑鼠\t鼠标\n"), []byte("記憶體\t記憶體\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []dictfile.Entry{{Key: "记忆体", Values: []string{"記憶體"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if _, err := STPhrases(t2s, []byte("記憶體\tx\n"), []byte("記憶体\ty\n")); err == nil {
		t.Fatal("want conflict error")
	}
}

// TestUpToDate guards the committed generated dictionaries:
// run `go generate` in the module root when it fails.
func TestUpToDate(t *testing.T) {
	files, err := Generate(os.DirFS("../../data/config"), os.DirFS("../../data/dictionary"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 {
		t.Fatalf("generated %d files, want 7", len(files))
	}
	for name, want := range files {
		got, err := os.ReadFile("../../data/dictionary/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run go generate", name)
		}
	}
}
