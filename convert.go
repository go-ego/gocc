package gocc

import "unicode/utf8"

// convert appends s converted by m to dst. Text no key matches is copied
// as is; a complete ideographic description sequence (e.g. "⿰氵青") is
// copied whole so its components are not converted one by one.
func convert(dst []byte, m matcher, s []byte) []byte {
	start := 0 // start of the pending unmatched run
	for i := 0; i < len(s); {
		r, size := runeAt(s[i:])
		if n, v := m.match(s[i:], r); n > 0 {
			dst = append(dst, s[start:i]...)
			dst = append(dst, v...)
			i += n
			start = i
			continue
		}
		i += skipLen(s[i:], r, size)
	}
	return append(dst, s[start:]...)
}

// segment calls fn with the segments of s found by maximum matching against
// m: each matched key is a segment, and so is each run of unmatched text.
func segment(m matcher, s []byte, fn func(seg []byte)) {
	start := 0
	for i := 0; i < len(s); {
		r, size := runeAt(s[i:])
		n, _ := m.match(s[i:], r)
		if n == 0 {
			i += skipLen(s[i:], r, size)
			continue
		}
		if start < i {
			fn(s[start:i])
		}
		fn(s[i : i+n])
		i += n
		start = i
	}
	if start < len(s) {
		fn(s[start:])
	}
}

// skipLen is the length of the unmatched unit at the head of s: a complete
// ideographic description sequence or else its first rune r of size bytes.
func skipLen(s []byte, r rune, size int) int {
	if idsArity(r) == 0 {
		return size
	}
	runes := 0
	if n, ok := idsLen(s, 16, &runes); ok {
		return n
	}
	return size
}

// idsLen parses an ideographic description sequence (or a single rune when
// s does not start with an operator) within OpenCC's depth and size limits.
func idsLen(s []byte, depth int, runes *int) (int, bool) {
	if len(s) == 0 || depth == 0 || *runes >= 64 {
		return 0, false
	}
	r, size := utf8.DecodeRune(s)
	if r == utf8.RuneError && size <= 1 {
		return 0, false
	}
	*runes++
	if *runes == 1 && idsArity(r) == 0 {
		return 0, false // not a sequence; the caller takes one rune
	}
	off := size
	for range idsArity(r) {
		n, ok := idsLen(s[off:], depth-1, runes)
		if !ok {
			return 0, false
		}
		off += n
	}
	return off, true
}

// idsArity is the number of operands of an ideographic description operator.
func idsArity(r rune) int {
	switch {
	case r == 0x2FF2 || r == 0x2FF3:
		return 3
	case r == 0x2FFE || r == 0x2FFF:
		return 1
	case r >= 0x2FF0 && r <= 0x2FFD:
		return 2
	}
	return 0
}

// normalize returns s converted by m, or s itself when nothing matches,
// so text needing no normalization is not copied.
func normalize(m matcher, s []byte) []byte {
	for i := 0; i < len(s); {
		r, size := runeAt(s[i:])
		if n, _ := m.match(s[i:], r); n > 0 {
			return convert(append(make([]byte, 0, len(s)), s[:i]...), m, s[i:])
		}
		i += skipLen(s[i:], r, size)
	}
	return s
}
