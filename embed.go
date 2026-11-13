package gocc

import (
	"embed"
	"io/fs"
	"sync"
)

//go:generate go run ./internal/cmd/gendict

// The configs are small and always embedded, so New works without the
// embedded dictionaries given LoadDict; see dict_embed.go.
//
//go:embed data/config/*.json
var configFS embed.FS

// builtin loads the embedded dictionaries once for all converters. Built
// with the "ne" tag it has none.
var builtin = sync.OnceValue(func() *loader {
	var dirs []fs.FS
	if dicts := embeddedDicts(); dicts != nil {
		dirs = append(dirs, dicts)
	}
	return newLoader(nil, dirs...)
})
