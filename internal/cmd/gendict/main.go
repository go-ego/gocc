// Command gendict refreshes the embedded OpenCC data: it optionally syncs
// configs and source dictionaries from an OpenCC checkout, then writes the
// generated dictionaries into data/dictionary.
//
//	go run ./internal/cmd/gendict [-opencc /path/to/OpenCC] [-data data]
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-ego/gocc/internal/dictgen"
)

func main() {
	opencc := flag.String("opencc", "", "OpenCC source checkout to sync configs and dictionaries from")
	data := flag.String("data", "data", "gocc data directory")
	flag.Parse()

	configDir := filepath.Join(*data, "config")
	dictDir := filepath.Join(*data, "dictionary")
	if *opencc != "" {
		if err := sync(filepath.Join(*opencc, "data", "config", "*.json"), configDir); err != nil {
			log.Fatal(err)
		}
		if err := sync(filepath.Join(*opencc, "data", "dictionary", "*.txt"), dictDir); err != nil {
			log.Fatal(err)
		}
	}

	files, err := dictgen.Generate(os.DirFS(configDir), os.DirFS(dictDir))
	if err != nil {
		log.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dictDir, name), content, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s", name)
	}
}

// sync copies the files matching pattern into dir, skipping JSON schemas.
func sync(pattern, dir string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, f := range files {
		if strings.Contains(filepath.Base(f), "schema") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(f)), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}
