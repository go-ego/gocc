// Command gocc converts Chinese text between Traditional, Simplified and
// regional variants, like the opencc command.
//
//	echo "不能踏入同一条河流" | gocc -c s2t
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	gocc "github.com/go-ego/gocc"
)

func main() {
	err := run(os.Args[1:], os.Stdin, os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gocc:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	fl := flag.NewFlagSet("gocc", flag.ContinueOnError)
	conv := fl.String("c", "s2t", "conversion or OpenCC config file: "+strings.Join(gocc.Conversions(), ", "))
	dictPath := fl.String("path", "", "directory searched for dictionaries before the built-in ones")
	in := fl.String("i", "", "input file (default stdin)")
	out := fl.String("o", "", "output file (default stdout)")
	tofu := fl.Bool("tofu", false, "include dictionaries that may output characters common fonts lack")
	if err := fl.Parse(args); err != nil {
		return err
	}

	var opts []gocc.Option
	if *tofu {
		opts = append(opts, gocc.WithTofuRisk())
	}
	if *dictPath != "" {
		opts = append(opts, gocc.LoadDict(*dictPath))
	}
	newConverter := gocc.New
	if isConfigFile(*conv) {
		newConverter = gocc.NewFromFile
	}
	c, err := newConverter(*conv, opts...)
	if err != nil {
		return err
	}

	if *in != "" {
		f, err := os.Open(*in)
		if err != nil {
			return err
		}
		defer f.Close()
		stdin = f
	}
	if *out == "" {
		return convert(c, stdin, stdout)
	}
	// creating the output truncates it, so it must not be the input
	if *in != "" && sameFile(*in, *out) {
		return fmt.Errorf("input and output are the same file: %s", *out)
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err := convert(c, stdin, f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// isConfigFile reports whether conv names an existing config file, which
// like opencc takes precedence over a built-in conversion.
func isConfigFile(conv string) bool {
	if !strings.HasSuffix(conv, ".json") {
		return false
	}
	fi, err := os.Stat(conv)
	return err == nil && fi.Mode().IsRegular()
}

func sameFile(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}

// convert streams r to w line by line; no dictionary key spans a newline.
func convert(c *gocc.Converter, r io.Reader, w io.Writer) error {
	br := bufio.NewReaderSize(r, 64<<10)
	bw := bufio.NewWriterSize(w, 64<<10)
	var buf []byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			buf = c.AppendConvert(buf[:0], line)
			if _, werr := bw.Write(buf); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return bw.Flush()
		}
		if err != nil {
			return err
		}
	}
}
