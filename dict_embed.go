//go:build !ne

package gocc

import (
	"embed"
	"io/fs"
)

//go:embed data/dictionary/*.txt
var dictFS embed.FS

// embeddedDicts returns the dictionaries compiled into the binary. Build
// with -tags ne to leave them out, like gse, and load them with
// LoadDict or NewFromFile.
func embeddedDicts() fs.FS {
	sub, err := fs.Sub(dictFS, "data/dictionary")
	if err != nil {
		panic(err) // the embedded directory always exists
	}
	return sub
}
