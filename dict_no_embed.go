//go:build ne

package gocc

import "io/fs"

// embeddedDicts returns nil: built with the "ne" tag, the binary has no
// dictionaries; load them with LoadDict or NewFromFile.
func embeddedDicts() fs.FS { return nil }
