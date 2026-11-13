package gocc

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/vcaesar/cedar"

	"github.com/go-ego/gocc/internal/dictfile"
)

// matcher finds the longest dictionary key that prefixes s.
type matcher interface {
	// match returns the key length in bytes (0 when nothing matches) and
	// the default value of that key; r is the first rune of s (see runeAt).
	match(s []byte, r rune) (int, string)
	// starts holds the first rune of every key.
	starts() *runeSet
}

// runeSet is a set of runes: a bitmap for the BMP, a map above it.
type runeSet struct {
	bmp    [0x10000 / 64]uint64
	astral map[rune]struct{}
}

func (s *runeSet) add(r rune) {
	if r < 0x10000 {
		s.bmp[r>>6] |= 1 << (r & 63)
		return
	}
	if s.astral == nil {
		s.astral = make(map[rune]struct{})
	}
	s.astral[r] = struct{}{}
}

func (s *runeSet) has(r rune) bool {
	if r < 0x10000 {
		return s.bmp[r>>6]&(1<<(r&63)) != 0
	}
	_, ok := s.astral[r]
	return ok
}

func (s *runeSet) union(o *runeSet) {
	for i, w := range o.bmp {
		s.bmp[i] |= w
	}
	for r := range o.astral {
		s.add(r)
	}
}

// runeAt decodes the rune at the head of non-empty s and its size; an
// invalid byte decodes as utf8.RuneError of size 1.
func runeAt(s []byte) (r rune, size int) {
	r, size = rune(s[0]), 1
	if r >= utf8.RuneSelf {
		r, size = decodeRune(s)
	}
	return
}

// decodeRune is runeAt past ASCII, apart so that runeAt inlines. Lead
// bytes E1-EC, which most CJK text has, admit any two continuation bytes.
func decodeRune(s []byte) (rune, int) {
	if b := s[0]; b >= 0xE1 && b <= 0xEC && len(s) >= 3 && s[1]&0xC0 == 0x80 && s[2]&0xC0 == 0x80 {
		return rune(b&0x0F)<<12 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F), 3
	}
	return utf8.DecodeRune(s)
}

// trie is a leaf dictionary backed by a cedar double-array trie;
// trie values index vals.
type trie struct {
	cd    *cedar.Cedar
	vals  []string
	first runeSet
}

// newTrie builds a trie from entry lists; when several lists hold the same
// key the earliest one wins, which is OpenCC's union tie-break.
func newTrie(lists ...[]dictfile.Entry) (*trie, error) {
	t := &trie{cd: cedar.New()}
	for _, entries := range lists {
		for _, e := range entries {
			// match walks whole runes, and cedar uses label 0 as its
			// terminal edge
			if !utf8.ValidString(e.Key) || strings.IndexByte(e.Key, 0) >= 0 {
				return nil, fmt.Errorf("gocc: dict key is not valid UTF-8 or contains NUL: %q", e.Key)
			}
			key := []byte(e.Key)
			if _, err := t.cd.Get(key); err == nil {
				continue // an earlier entry has the key
			}
			if err := t.cd.Insert(key, len(t.vals)); err != nil {
				return nil, err
			}
			t.vals = append(t.vals, e.Values[0])
			r, _ := utf8.DecodeRuneInString(e.Key)
			t.first.add(r)
		}
	}
	return t, nil
}

func (t *trie) starts() *runeSet { return &t.first }

func (t *trie) match(s []byte, r rune) (int, string) {
	if !t.first.has(r) {
		return 0, ""
	}
	n, val := 0, -1
	// walk one rune per Jump, sized by its lead byte: keys are valid UTF-8,
	// so values sit only at rune ends and invalid input just fails to walk.
	// A NUL would follow cedar's terminal edge and fake a match.
	for i, from := 0, 0; i < len(s) && s[i] != 0; {
		end := min(i+leadLen(s[i]), len(s))
		to, err := t.cd.Jump(s[i:end], from)
		if err != nil {
			break
		}
		from, i = to, end
		if v, err := t.cd.Value(to); err == nil {
			n, val = i, v
		}
	}
	if val < 0 {
		return 0, ""
	}
	return n, t.vals[val]
}

// leadLen is the UTF-8 sequence length announced by lead byte b
// (1 for ASCII and stray continuation bytes).
func leadLen(b byte) int {
	switch {
	case b < 0xC0:
		return 1
	case b < 0xE0:
		return 2
	case b < 0xF0:
		return 3
	}
	return 4
}

// shortCircuit returns the match of the first dictionary that matches.
type shortCircuit struct {
	ms    []matcher
	first runeSet
}

// union returns the longest match among its dictionaries, the earliest
// dictionary winning ties.
type union struct {
	ms    []matcher
	first runeSet
}

func newShortCircuit(ms []matcher) *shortCircuit {
	g := &shortCircuit{ms: ms}
	for _, m := range ms {
		g.first.union(m.starts())
	}
	return g
}

func newUnion(ms []matcher) *union {
	g := &union{ms: ms}
	for _, m := range ms {
		g.first.union(m.starts())
	}
	return g
}

func (g *shortCircuit) starts() *runeSet { return &g.first }

func (g *union) starts() *runeSet { return &g.first }

func (g *shortCircuit) match(s []byte, r rune) (int, string) {
	if !g.first.has(r) {
		return 0, ""
	}
	for _, m := range g.ms {
		if n, v := m.match(s, r); n > 0 {
			return n, v
		}
	}
	return 0, ""
}

func (g *union) match(s []byte, r rune) (int, string) {
	if !g.first.has(r) {
		return 0, ""
	}
	best, val := 0, ""
	for _, m := range g.ms {
		if n, v := m.match(s, r); n > best {
			best, val = n, v
		}
	}
	return best, val
}
