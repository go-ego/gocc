//go:build ne

package gocc

import "testing"

// TestNoEmbed runs with -tags ne: the binary has no dictionaries, which
// LoadDict provides.
func TestNoEmbed(t *testing.T) {
	if _, err := New("s2t"); err == nil {
		t.Fatal("want an error without embedded dictionaries")
	}
	c, err := New("s2t", LoadDict("data/dictionary"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Convert("汉字"), "漢字"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
