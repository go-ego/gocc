// Package dictfile reads and writes OpenCC text dictionaries.
//
// Each non-empty line not starting with '#' is "key<TAB>value[ value...]".
// The first value is the default conversion.
package dictfile

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Entry is one dictionary line.
type Entry struct {
	Key    string
	Values []string
}

// Parse parses a text dictionary, rejecting malformed lines and duplicate keys.
func Parse(data []byte) ([]Entry, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var entries []Entry
	seen := make(map[string]struct{})
	n := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		n++
		line = strings.TrimSuffix(line, "\r")
		if line == "" || line[0] == '#' {
			continue
		}
		key, rest, ok := strings.Cut(line, "\t")
		if !ok || key == "" {
			return nil, fmt.Errorf("line %d: missing key or tab: %q", n, line)
		}
		// like OpenCC, values are separated by single spaces only
		values := strings.Split(rest, " ")
		if values[0] == "" {
			return nil, fmt.Errorf("line %d: no value for key %q", n, key)
		}
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("line %d: duplicate key %q", n, key)
		}
		seen[key] = struct{}{}
		entries = append(entries, Entry{Key: key, Values: values})
	}
	return entries, nil
}

// Format renders entries sorted by key, preceded by the header lines
// (each written as a "# " comment).
func Format(header []string, entries []Entry) []byte {
	sorted := append([]Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	var b bytes.Buffer
	for _, h := range header {
		b.WriteString("# ")
		b.WriteString(h)
		b.WriteString("\n")
	}
	for _, e := range sorted {
		b.WriteString(e.Key)
		b.WriteString("\t")
		b.WriteString(strings.Join(e.Values, " "))
		b.WriteString("\n")
	}
	return b.Bytes()
}
