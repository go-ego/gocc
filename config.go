package gocc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/go-ego/gocc/internal/dictfile"
)

// configSpec mirrors an OpenCC JSON configuration.
type configSpec struct {
	Name            string            `json:"name"`
	Normalization   []conversionSpec  `json:"normalization"`
	Segmentation    *segmentSpec      `json:"segmentation"`
	ConversionChain *[]conversionSpec `json:"conversion_chain"`
}

type segmentSpec struct {
	Type string    `json:"type"`
	Dict *dictSpec `json:"dict"`
}

type conversionSpec struct {
	Dict *dictSpec `json:"dict"`
}

type dictSpec struct {
	Type          string        `json:"type"`
	File          string        `json:"file"`
	MatchPolicy   *string       `json:"match_policy"`
	Dicts         []dictSpec    `json:"dicts"`
	Entries       inlineEntries `json:"entries"`
	MayOutputTofu *bool         `json:"may_output_tofu"`
}

// excluded reports whether d is dropped for its tofu risk.
func (d *dictSpec) excluded(tofu bool) bool {
	return d.MayOutputTofu != nil && *d.MayOutputTofu && !tofu
}

// inlineEntries keeps the entries of an inline dict in document order and
// rejects duplicate keys, which a Go map would silently merge.
type inlineEntries []dictfile.Entry

func (e *inlineEntries) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return errors.New("gocc: inline entries must be an object")
	}
	seen := make(map[string]struct{})
	out := inlineEntries{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key := tok.(string) // object keys are always strings
		var val string
		if err := dec.Decode(&val); err != nil {
			return fmt.Errorf("gocc: inline value of %q: %w", key, err)
		}
		if key == "" || val == "" {
			return fmt.Errorf("gocc: inline dict has an empty key or value: %q", key)
		}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("gocc: inline dict has duplicate key %q", key)
		}
		seen[key] = struct{}{}
		out = append(out, dictfile.Entry{Key: key, Values: []string{val}})
	}
	*e = out
	return nil
}

// loader builds matchers from dictionary files and caches the built tries,
// so converters sharing a loader share memory. A file is searched in each
// of fsys in order, then in next, whose cache is shared.
type loader struct {
	fsys []fs.FS
	next *loader

	mu    sync.Mutex
	tries map[string]*trie
}

func newLoader(next *loader, fsys ...fs.FS) *loader {
	return &loader{
		fsys:  fsys,
		next:  next,
		tries: make(map[string]*trie),
	}
}

// build compiles a config; tofu includes dictionaries marked may_output_tofu.
func (l *loader) build(data []byte, tofu bool) (*Converter, error) {
	var spec configSpec
	// like OpenCC, accept comments and trailing commas
	if err := json.Unmarshal(stripJSONC(data), &spec); err != nil {
		return nil, fmt.Errorf("gocc: parse config: %w", err)
	}
	if spec.ConversionChain == nil {
		return nil, errors.New("gocc: config has no conversion_chain")
	}

	c := &Converter{name: spec.Name}
	var err error
	if c.norm, err = l.chain(spec.Normalization, tofu); err != nil {
		return nil, err
	}
	if c.chain, err = l.chain(*spec.ConversionChain, tofu); err != nil {
		return nil, err
	}
	if s := spec.Segmentation; s != nil {
		if s.Type != "mmseg" {
			return nil, fmt.Errorf("gocc: unsupported segmentation type %q", s.Type)
		}
		if s.Dict == nil {
			return nil, errors.New("gocc: segmentation has no dict")
		}
		// like OpenCC, segmentation always sees the tofu-risk dictionaries
		if c.seg, err = l.dict(s.Dict, true); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// chain builds the conversions, dropping those left without a dictionary.
func (l *loader) chain(specs []conversionSpec, tofu bool) ([]matcher, error) {
	var ms []matcher
	for _, s := range specs {
		if s.Dict == nil {
			return nil, errors.New("gocc: conversion has no dict")
		}
		m, err := l.dict(s.Dict, tofu)
		if err != nil {
			return nil, err
		}
		if m != nil {
			ms = append(ms, m)
		}
	}
	return ms, nil
}

// dict builds the matcher for d; it is nil when every dictionary is excluded.
func (l *loader) dict(d *dictSpec, tofu bool) (matcher, error) {
	if d.Type == "inline" {
		return inlineDict(d)
	}
	// like Config.cpp, an excluded dictionary is not validated further
	if d.excluded(tofu) {
		return nil, nil
	}
	switch d.Type {
	case "text", "ocd", "ocd2":
		if d.File == "" {
			return nil, fmt.Errorf("gocc: %s dict has no file", d.Type)
		}
		return l.trie([]string{txtName(d.File)})
	case "group":
		return l.group(d, tofu)
	}
	return nil, fmt.Errorf("gocc: unknown dict type %q", d.Type)
}

func inlineDict(d *dictSpec) (matcher, error) {
	if d.MayOutputTofu != nil {
		return nil, errors.New("gocc: inline dict does not support may_output_tofu")
	}
	if d.Entries == nil {
		return nil, errors.New("gocc: inline dict has no entries")
	}
	return newTrie(d.Entries)
}

func (l *loader) group(d *dictSpec, tofu bool) (matcher, error) {
	if d.Dicts == nil {
		return nil, errors.New("gocc: group dict has no dicts")
	}
	policy := "short_circuit"
	if d.MatchPolicy != nil {
		policy = *d.MatchPolicy
	}
	if policy != "short_circuit" && policy != "union" {
		return nil, fmt.Errorf("gocc: unknown match_policy %q", policy)
	}

	// a union of plain files is one trie: a single walk finds the longest key
	if files, ok := unionFiles(d, tofu); ok && policy == "union" {
		if len(files) == 0 {
			return nil, nil
		}
		return l.trie(files)
	}

	var ms []matcher
	for i := range d.Dicts {
		m, err := l.dict(&d.Dicts[i], tofu)
		if err != nil {
			return nil, err
		}
		if m != nil {
			ms = append(ms, m)
		}
	}
	switch {
	case len(ms) == 0:
		return nil, nil
	case len(ms) == 1:
		return ms[0], nil
	case policy == "union":
		return newUnion(ms), nil
	}
	return newShortCircuit(ms), nil
}

// unionFiles lists, in order, the files of a group tree made only of union
// groups and file dictionaries; ok is false for any other shape, which
// dict then builds or rejects.
func unionFiles(d *dictSpec, tofu bool) (files []string, ok bool) {
	if d.Type == "inline" {
		return nil, false
	}
	if d.excluded(tofu) {
		return nil, true
	}
	switch d.Type {
	case "text", "ocd", "ocd2":
		return []string{txtName(d.File)}, d.File != ""
	case "group":
		if d.Dicts == nil || d.MatchPolicy == nil || *d.MatchPolicy != "union" {
			return nil, false
		}
		for i := range d.Dicts {
			sub, ok := unionFiles(&d.Dicts[i], tofu)
			if !ok {
				return nil, false
			}
			files = append(files, sub...)
		}
		return files, true
	}
	return nil, false
}

// trie returns the cached trie merging files, the earliest file winning
// duplicate keys.
func (l *loader) trie(files []string) (*trie, error) {
	if l.next != nil && !l.hasAny(files) {
		return l.next.trie(files)
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	key := strings.Join(files, "\x00")
	if t, ok := l.tries[key]; ok {
		return t, nil
	}
	lists := make([][]dictfile.Entry, len(files))
	for i, f := range files {
		entries, err := l.load(f)
		if err != nil {
			return nil, err
		}
		lists[i] = entries
	}
	t, err := newTrie(lists...)
	if err != nil {
		return nil, err
	}
	l.tries[key] = t
	return t, nil
}

// hasAny reports whether any of files may be in l's own file systems.
func (l *loader) hasAny(files []string) bool {
	for _, f := range files {
		for _, fsys := range l.fsys {
			if _, err := fs.Stat(fsys, f); !errors.Is(err, fs.ErrNotExist) {
				return true // found, or an error load reports
			}
		}
	}
	return false
}

// txtName maps binary OpenCC dictionary names (.ocd2, .ocd) to their text
// source (.txt).
func txtName(file string) string {
	for _, ext := range []string{".ocd2", ".ocd"} {
		if strings.HasSuffix(file, ext) {
			return strings.TrimSuffix(file, ext) + ".txt"
		}
	}
	return file
}

// load parses a text dictionary. The entries are not kept: a file goes
// into one trie in practice, which is what the loader caches.
func (l *loader) load(name string) ([]dictfile.Entry, error) {
	err := fmt.Errorf("gocc: load dict %s: no dictionary file system (built with -tags ne? use LoadDict)", name)
	for _, fsys := range l.fsys {
		data, rerr := fs.ReadFile(fsys, name)
		if rerr == nil {
			entries, perr := dictfile.Parse(data)
			if perr != nil {
				return nil, fmt.Errorf("gocc: dict %s: %w", name, perr)
			}
			return entries, nil
		}
		err = fmt.Errorf("gocc: load dict %s: %w", name, rerr)
		if !errors.Is(rerr, fs.ErrNotExist) {
			return nil, err
		}
	}
	if l.next != nil {
		return l.next.load(name)
	}
	return nil, err
}

// stripJSONC blanks // and /* */ comments outside strings and drops
// trailing commas before '}' or ']'.
func stripJSONC(data []byte) []byte {
	out := make([]byte, 0, len(data))
	comma := -1    // index in out of a pending comma
	value := false // the last token ends a value, which a comma may follow
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case c == '"':
			comma, value = -1, true
			j := i + 1
			for ; j < len(data) && data[j] != '"'; j++ {
				if data[j] == '\\' {
					j++
				}
			}
			j = min(j, len(data)-1)
			out = append(out, data[i:j+1]...)
			i = j
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			i--
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				return append(out, data[i:]...) // let json report the error
			}
			i += end + 3
		case c == ',':
			comma = -1
			if value { // "[,]" and "[1,,]" stay invalid
				comma = len(out)
			}
			value = false
			out = append(out, c)
		case c == '}' || c == ']':
			if comma >= 0 {
				out[comma] = ' '
			}
			comma, value = -1, true
			out = append(out, c)
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			out = append(out, c)
		default:
			comma, value = -1, c != '[' && c != '{' && c != ':'
			out = append(out, c)
		}
	}
	return out
}
