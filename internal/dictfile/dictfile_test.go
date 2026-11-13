package dictfile

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	data := "\xef\xbb\xbf# comment\n\n干\t幹 乾 干\r\n" + "面条\t麵條\n"
	got, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Key: "干", Values: []string{"幹", "乾", "干"}},
		{Key: "面条", Values: []string{"麵條"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestParseSeparators follows OpenCC: only ' ' separates values.
func TestParseSeparators(t *testing.T) {
	got, err := Parse([]byte("A\tX\u3000Y\nB\tP\tQ\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Values[0] != "X\u3000Y" || got[1].Values[0] != "P\tQ" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"a b\n", "\tx\n", "a\t \n", "a\tb\na\tc\n"} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) want error", in)
		}
	}
}

func TestFormat(t *testing.T) {
	out := Format([]string{"h"}, []Entry{
		{Key: "b", Values: []string{"1"}},
		{Key: "a", Values: []string{"2", "3"}},
	})
	if want := "# h\na\t2 3\nb\t1\n"; string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	back, err := Parse(out)
	if err != nil || len(back) != 2 || back[0].Key != "a" {
		t.Fatalf("round trip: %+v %v", back, err)
	}
}
