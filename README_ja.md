
# gocc

[![Build Status](https://github.com/go-ego/gocc/workflows/Go/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/go.yml)
[![Lint Status](https://github.com/go-ego/gocc/workflows/Lint/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ego/gocc.svg)](https://pkg.go.dev/github.com/go-ego/gocc)

[English](https://github.com/go-ego/gocc/blob/main/README.md) | [简体中文](https://github.com/go-ego/gocc/blob/main/README_zh.md) | [繁體中文](https://github.com/go-ego/gocc/blob/main/README_zht.md)

`gocc` は [OpenCC](https://github.com/BYVoid/OpenCC)
（Open Chinese Convert 開放中文轉換）の Go 実装です。繁体字中国語、
簡体字中国語、および地域ごとの字体・語彙の間で変換します。

OpenCC 1.4 の変換モデルに従い、その設定と辞書を同梱しています。
辞書は [cedar](https://github.com/vcaesar/cedar) のダブル配列トライに格納されます。

## インストール

```sh
go get github.com/go-ego/gocc
go install github.com/go-ego/gocc/cmd/gocc@latest
```

## 使用方法

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

カスタム設定（OpenCC JSON。`X.ocd2`/`X.ocd` 辞書は `X.txt` として読み込まれます）:

```go
c, err := gocc.NewFromConfig(configJSON, os.DirFS("dicts"))
```

設定ファイル（OpenCC の `SimpleConverter` と同様）: 辞書は設定ファイルのディレクトリ、
`LoadDict` のディレクトリ、組み込み辞書の順に検索されます:

```go
c, err := gocc.NewFromFile("my/s2t_custom.json")
```

辞書ディレクトリ（OpenCC の `--path` や gse の `LoadDict` と同様）:
そこにあるファイルが同名の組み込み辞書を置き換えます:

```go
c, err := gocc.New("s2t", gocc.LoadDict("my/dicts")) // 例: my/dicts/STPhrases.txt
```

gse と同様に、`-tags ne` を付けてビルドすると辞書（約 1.5 MB）をバイナリに含めません。
設定は埋め込まれたままなので、辞書は `LoadDict` で読み込みます:

```sh
go build -tags ne ./cmd/gocc && gocc -c s2t -path /usr/share/gocc/dictionary
```

`Simplified()`、`Traditional()`、`JP()` はコンバーターの出力の種類を返します。
設定名から判定され、`t2s` は簡体字、`s2twp` は繁体字、`t2jp` は日本語です。

OpenCC と同様に、設定にはコメントと末尾のカンマを含めることができます。

`gocc.WithTofuRisk()` は `may_output_tofu` とマークされた辞書も使用します。
これらの辞書が出力する珍しい文字は、一般的なフォントでは表示できないことがあります
（OpenCC の `--include-tofu-risk-dictionaries` に相当）。

コマンドライン:

```sh
echo "不能踏入同一条河流" | gocc -c s2t
# 不能踏入同一條河流
gocc -c t2s -i in.txt -o out.txt
gocc -c my/config.json -path my/dicts  # 設定ファイルと辞書ディレクトリ
```

特殊な入力に対する動作: 不正な UTF-8 バイトと NUL バイトはそのまま出力されます
（OpenCC は不正な UTF-8 に対して例外を送出します）。`AppendConvert` の `dst` は
入力と重なってはいけません。

## 変換

| 名前                         | 変換                                                                   |
| ---------------------------- | ---------------------------------------------------------------------- |
| `s2t`                        | 簡体字中国語から繁体字中国語へ                                         |
| `t2s`                        | 繁体字中国語から簡体字中国語へ                                         |
| `s2tw`                       | 簡体字中国語から繁体字中国語（台湾標準）へ                             |
| `tw2s`                       | 繁体字中国語（台湾標準）から簡体字中国語へ                             |
| `s2hk`                       | 簡体字中国語から繁体字中国語（香港異体字）へ                           |
| `hk2s`                       | 繁体字中国語（香港異体字）から簡体字中国語へ                           |
| `s2twp`                      | 簡体字中国語から繁体字中国語（台湾標準）へ、台湾の常用語彙に変換       |
| `tw2sp`                      | 繁体字中国語（台湾標準）から簡体字中国語へ、中国大陸の常用語彙に変換   |
| `s2hkp`                      | 簡体字中国語から繁体字中国語（香港異体字）へ、香港の常用語彙に変換     |
| `hk2sp`                      | 繁体字中国語（香港異体字）から簡体字中国語へ、中国大陸の常用語彙に変換 |
| `t2tw`                       | 繁体字中国語（OpenCC 標準）から台湾標準へ                              |
| `tw2t`                       | 繁体字中国語（台湾標準）から繁体字中国語（OpenCC 標準）へ              |
| `t2hk`                       | 繁体字中国語（OpenCC 標準）から香港異体字へ                            |
| `hk2t`                       | 繁体字中国語（香港異体字）から繁体字中国語（OpenCC 標準）へ            |
| `t2jp`                       | 繁体字中国語の漢字（旧字体）から日本語の新字体へ                       |
| `jp2t`                       | 日本語の新字体から繁体字中国語の漢字（旧字体）へ                       |
| `s2seal`, `t2seal`, `seal2t` | 篆書体の変換                                                           |

## データの更新

`data/` には OpenCC の `data/config/*.json` と `data/dictionary/*.txt`、
および OpenCC がビルド時に生成する辞書（`*Rev.txt`、
`TSCharactersExt.txt`、`STPhrases_GeneratedFromRegionalPhrases.txt`）が入っています:

```sh
go run ./internal/cmd/gendict -opencc /path/to/OpenCC   # 同期して再生成
go generate                                              # 再生成のみ
```

## Lint

コードは CI で [golangci-lint](https://golangci-lint.run) v2 によりチェックされます。
設定は [.golangci.yml](.golangci.yml) にあります:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run ./...
golangci-lint run --build-tags ne ./...
```

## ライセンス

Apache-2.0。辞書と設定は OpenCC（Apache-2.0）に由来します。
