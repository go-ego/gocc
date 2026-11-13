# gocc

[![Build Status](https://github.com/go-ego/gocc/workflows/Go/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/go.yml)
[![Lint Status](https://github.com/go-ego/gocc/workflows/Lint/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ego/gocc.svg)](https://pkg.go.dev/github.com/go-ego/gocc)

[简体中文](https://github.com/go-ego/gocc/blob/main/README_zh.md) | [繁體中文](https://github.com/go-ego/gocc/blob/main/README_zht.md) | [日本語](https://github.com/go-ego/gocc/blob/main/README_ja.md)

Package `gocc` is a Go implementation of [OpenCC](https://github.com/BYVoid/OpenCC)
(Open Chinese Convert 開放中文轉換): conversion between Traditional Chinese,
Simplified Chinese and regional variants.

It follows the OpenCC 1.4 conversion model and ships its configs and
dictionaries, stored in [cedar](https://github.com/vcaesar/cedar)
double-array tries.

## Install

```sh
go get github.com/go-ego/gocc
go install github.com/go-ego/gocc/cmd/gocc@latest
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ego/gocc"
)

func main() {
	s2t, err := gocc.New("s2t")
	if err != nil {
		panic(err)
	}
	fmt.Println(s2t.Convert("那里湖面总是澄清, 那里空气充满宁静"))
	// 那裏湖面總是澄清, 那裏空氣充滿寧靜

	fmt.Println(gocc.MustNew("s2twp").Convert("鼠标和内存"))
	// 滑鼠和記憶體
}
```

Custom config (OpenCC JSON; `X.ocd2`/`X.ocd` dictionaries are read as `X.txt`):

```go
c, err := gocc.NewFromConfig(configJSON, os.DirFS("dicts"))
```

Config file, like OpenCC's `SimpleConverter`: dictionaries are searched in the
config's directory, then `LoadDict` directories, then the built-in ones:

```go
c, err := gocc.NewFromFile("my/s2t_custom.json")
```

Dictionary directory, like OpenCC's `--path` or gse's `LoadDict`: files found
there replace the built-in dictionaries of the same name:

```go
c, err := gocc.New("s2t", gocc.LoadDict("my/dicts")) // e.g. my/dicts/STPhrases.txt
```

Build with `-tags ne` to leave the dictionaries (~1.5 MB) out of the binary,
like gse; the configs stay embedded, so load the dictionaries with `LoadDict`:

```sh
go build -tags ne ./cmd/gocc && gocc -c s2t -path /usr/share/gocc/dictionary
```

`Simplified()`, `Traditional()` and `JP()` report the converter's output, read
from its config name: `t2s` is Simplified, `s2twp` Traditional, `t2jp` JP.

Configs may contain comments and trailing commas, as OpenCC allows.

`gocc.WithTofuRisk()` also uses dictionaries marked `may_output_tofu`, whose
rare characters common fonts may not render (OpenCC's
`--include-tofu-risk-dictionaries`).

Command line:

```sh
echo "不能踏入同一条河流" | gocc -c s2t
# 不能踏入同一條河流
gocc -c t2s -i in.txt -o out.txt
gocc -c my/config.json -path my/dicts  # config file and dictionary directory
```

Behaviour on unusual input: invalid UTF-8 bytes and NUL bytes pass through
unchanged (OpenCC throws on invalid UTF-8). `AppendConvert`'s `dst` must
not overlap its input.

## Conversions

| Name                         | Conversion                                                                         |
| ---------------------------- | ---------------------------------------------------------------------------------- |
| `s2t`                        | Simplified Chinese to Traditional Chinese                                          |
| `t2s`                        | Traditional Chinese to Simplified Chinese                                          |
| `s2tw`                       | Simplified Chinese to Traditional Chinese (Taiwan Standard)                        |
| `tw2s`                       | Traditional Chinese (Taiwan Standard) to Simplified Chinese                        |
| `s2hk`                       | Simplified Chinese to Traditional Chinese (Hong Kong variant)                      |
| `hk2s`                       | Traditional Chinese (Hong Kong variant) to Simplified Chinese                      |
| `s2twp`                      | Simplified Chinese to Traditional Chinese (Taiwan Standard) with Taiwanese idiom   |
| `tw2sp`                      | Traditional Chinese (Taiwan Standard) to Simplified Chinese with Mainland idiom    |
| `s2hkp`                      | Simplified Chinese to Traditional Chinese (Hong Kong variant) with Hong Kong idiom |
| `hk2sp`                      | Traditional Chinese (Hong Kong variant) to Simplified Chinese with Mainland idiom  |
| `t2tw`                       | Traditional Chinese (OpenCC Standard) to Taiwan Standard                           |
| `tw2t`                       | Traditional Chinese (Taiwan Standard) to Traditional Chinese (OpenCC Standard)     |
| `t2hk`                       | Traditional Chinese (OpenCC Standard) to Hong Kong variant                         |
| `hk2t`                       | Traditional Chinese (Hong Kong variant) to Traditional Chinese (OpenCC Standard)   |
| `t2jp`                       | Traditional Chinese characters (Kyūjitai) to New Japanese Kanji (Shinjitai)        |
| `jp2t`                       | New Japanese Kanji (Shinjitai) to Traditional Chinese characters (Kyūjitai)        |
| `s2seal`, `t2seal`, `seal2t` | Seal script conversions                                                            |

## Updating the data

`data/` holds OpenCC's `data/config/*.json` and `data/dictionary/*.txt`
plus the dictionaries OpenCC generates at build time (`*Rev.txt`,
`TSCharactersExt.txt`, `STPhrases_GeneratedFromRegionalPhrases.txt`):

```sh
go run ./internal/cmd/gendict -opencc /path/to/OpenCC   # sync and regenerate
go generate                                              # regenerate only
```

## Lint

The code is checked by [golangci-lint](https://golangci-lint.run) v2 in CI, with
the config in [.golangci.yml](.golangci.yml):

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run ./...
golangci-lint run --build-tags ne ./...
```

## License

Apache-2.0. The dictionaries and configs come from OpenCC (Apache-2.0).
