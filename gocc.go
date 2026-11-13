// Package gocc is a Go implementation of OpenCC (Open Chinese Convert),
// converting between Traditional Chinese, Simplified Chinese and regional
// variants (Taiwan, Hong Kong, Japanese Shinjitai, seal script).
//
// It follows the OpenCC 1.4 conversion model: normalization, optional
// maximum-match segmentation, and a chain of dictionary conversions whose
// groups match with "short_circuit" or "union" policy. The OpenCC configs
// and dictionaries are embedded (build with -tags ne to leave the
// dictionaries out and use LoadDict); dictionaries are stored in cedar
// double-array tries and shared between converters.
package gocc

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unsafe"
)

// Converter converts text with one OpenCC configuration.
// It is safe for concurrent use.
type Converter struct {
	name  string
	norm  []matcher // normalization, applied to the whole text first
	seg   matcher   // segmentation dictionary; nil converts the text as one segment
	chain []matcher // conversions applied in order to every segment
}

// Option configures a Converter.
type Option func(*options)

type options struct {
	tofu bool
	dirs []fs.FS
}

// WithTofuRisk includes dictionaries marked "may_output_tofu": rare
// characters that common fonts may not render (OpenCC's
// --include-tofu-risk-dictionaries).
func WithTofuRisk() Option {
	return func(o *options) { o.tofu = true }
}

// LoadDict searches dir for dictionary files before the built-in
// dictionaries, like OpenCC's --path or gse's LoadDict. A file found in dir
// replaces the built-in one of the same name. Later paths are searched
// after earlier ones.
func LoadDict(dir string) Option {
	return func(o *options) { o.dirs = append(o.dirs, os.DirFS(dir)) }
}

func buildOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Conversions lists the built-in configurations, such as "s2t" and "tw2sp".
func Conversions() []string {
	files, err := fs.Glob(configFS, "data/config/*.json")
	if err != nil {
		panic(err) // the pattern is valid
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = strings.TrimSuffix(path.Base(f), ".json")
	}
	sort.Strings(names)
	return names
}

// New returns the converter of a built-in configuration such as "s2t"
// (Simplified to Traditional) or "t2s"; see Conversions. A ".json" suffix
// is accepted.
func New(conversion string, opts ...Option) (*Converter, error) {
	name := strings.TrimSuffix(conversion, ".json")
	if name == "" || strings.ContainsAny(name, `/\.`) {
		return nil, fmt.Errorf("gocc: unknown conversion %q", conversion)
	}
	data, err := configFS.ReadFile("data/config/" + name + ".json")
	if err != nil {
		return nil, fmt.Errorf("gocc: unknown conversion %q", conversion)
	}
	o := buildOptions(opts)
	l := builtin()
	if len(o.dirs) > 0 {
		l = newLoader(l, o.dirs...)
	}
	return l.build(data, o.tofu)
}

// NewFromFile builds a converter from an OpenCC JSON config file, like
// OpenCC's SimpleConverter. As in OpenCC, dictionary files are searched in
// the config's directory, then the LoadDict directories, then among
// the built-in dictionaries.
func NewFromFile(config string, opts ...Option) (*Converter, error) {
	data, err := os.ReadFile(config)
	if err != nil {
		return nil, fmt.Errorf("gocc: %w", err)
	}
	o := buildOptions(opts)
	dirs := append([]fs.FS{os.DirFS(filepath.Dir(config))}, o.dirs...)
	return newLoader(builtin(), dirs...).build(data, o.tofu)
}

// MustNew is like New but panics on error.
func MustNew(conversion string, opts ...Option) *Converter {
	c, err := New(conversion, opts...)
	if err != nil {
		panic(err)
	}
	return c
}

// NewFromConfig builds a converter from an OpenCC JSON config whose
// dictionary files are read from dicts. Text dictionaries are used for
// every dictionary type: "X.ocd2" and "X.ocd" are read as "X.txt".
// The LoadDict directories are searched after dicts.
func NewFromConfig(config []byte, dicts fs.FS, opts ...Option) (*Converter, error) {
	o := buildOptions(opts)
	var dirs []fs.FS
	if dicts != nil {
		dirs = append(dirs, dicts)
	}
	return newLoader(nil, append(dirs, o.dirs...)...).build(config, o.tofu)
}

// Name returns the configuration name, e.g.
// "Simplified Chinese to Traditional Chinese (OpenCC Standard)".
func (c *Converter) Name() string {
	return c.name
}

// Simplified reports whether the converter outputs Simplified Chinese,
// as "t2s" and "tw2sp" do. Like Traditional and JP, it reads the target
// from the config name, "... to Simplified Chinese ...".
func (c *Converter) Simplified() bool {
	return strings.Contains(c.target(), "Simplified Chinese")
}

// Traditional reports whether the converter outputs Traditional Chinese,
// as "s2t", "s2twp" and "seal2t" do.
func (c *Converter) Traditional() bool {
	return strings.Contains(c.target(), "Traditional Chinese")
}

// JP reports whether the converter outputs Japanese Kanji, as "t2jp"
// (Shinjitai) and "jp2t" (Kyūjitai) do.
func (c *Converter) JP() bool {
	return strings.Contains(c.target(), "Japanese")
}

// target returns the part of the name after the last " to ".
func (c *Converter) target() string {
	i := strings.LastIndex(c.name, " to ")
	if i < 0 {
		return ""
	}
	return c.name[i+len(" to "):]
}

// Convert converts s.
func (c *Converter) Convert(s string) string {
	// Chinese conversion rarely changes the length much
	dst := make([]byte, 0, len(s)+len(s)/8)
	// no copies: AppendConvert only reads s, and dst is not used again
	dst = c.AppendConvert(dst, unsafe.Slice(unsafe.StringData(s), len(s)))
	return unsafe.String(unsafe.SliceData(dst), len(dst))
}

// scratch holds the intermediate buffers of a conversion chain.
type scratch struct{ a, b []byte }

var scratchPool = sync.Pool{New: func() any { return new(scratch) }}

// AppendConvert appends the conversion of s to dst and returns the result.
// dst must not overlap s.
func (c *Converter) AppendConvert(dst, s []byte) []byte {
	for _, m := range c.norm {
		s = normalize(m, s)
	}
	if len(c.chain) < 2 {
		// no intermediate results
		return c.convertAll(dst, s, nil)
	}
	sc := scratchPool.Get().(*scratch)
	dst = c.convertAll(dst, s, sc)
	scratchPool.Put(sc)
	return dst
}

func (c *Converter) convertAll(dst, s []byte, sc *scratch) []byte {
	if c.seg == nil {
		return c.convertSegment(dst, s, sc)
	}
	segment(c.seg, s, func(seg []byte) {
		dst = c.convertSegment(dst, seg, sc)
	})
	return dst
}

// convertSegment runs the conversion chain over seg, using sc for the
// intermediate results.
func (c *Converter) convertSegment(dst, seg []byte, sc *scratch) []byte {
	if len(c.chain) == 0 {
		return append(dst, seg...)
	}
	last := len(c.chain) - 1
	for _, m := range c.chain[:last] {
		sc.a = convert(sc.a[:0], m, seg)
		seg = sc.a
		sc.a, sc.b = sc.b, sc.a
	}
	return convert(dst, c.chain[last], seg)
}
