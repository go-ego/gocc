
# gocc

[![Build Status](https://github.com/go-ego/gocc/workflows/Go/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/go.yml)
[![Lint Status](https://github.com/go-ego/gocc/workflows/Lint/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ego/gocc.svg)](https://pkg.go.dev/github.com/go-ego/gocc)

[English](https://github.com/go-ego/gocc/blob/main/README.md) | [简体中文](https://github.com/go-ego/gocc/blob/main/README_zh.md) | [日本語](https://github.com/go-ego/gocc/blob/main/README_ja.md)

`gocc` 是 [OpenCC](https://github.com/BYVoid/OpenCC)
(Open Chinese Convert 開放中文轉換) 的 Go 實作: 支援繁體中文、
簡體中文和地區用字用詞之間的轉換。

遵循 OpenCC 1.4 的轉換模型, 並內建其設定和詞典,
詞典用 [cedar](https://github.com/vcaesar/cedar) 雙陣列 trie 儲存。

## 安裝

```sh
go get github.com/go-ego/gocc
go install github.com/go-ego/gocc/cmd/gocc@latest
```

## 使用

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

自訂設定 (OpenCC JSON; `X.ocd2`/`X.ocd` 詞典按 `X.txt` 讀取):

```go
c, err := gocc.NewFromConfig(configJSON, os.DirFS("dicts"))
```

設定檔, 類似 OpenCC 的 `SimpleConverter`: 詞典先在設定檔所在的目錄中尋找,
然後是 `LoadDict` 指定的目錄, 最後是內建詞典:

```go
c, err := gocc.NewFromFile("my/s2t_custom.json")
```

詞典目錄, 類似 OpenCC 的 `--path` 或 gse 的 `LoadDict`:
該目錄下的檔案會替換同名的內建詞典:

```go
c, err := gocc.New("s2t", gocc.LoadDict("my/dicts")) // 例如 my/dicts/STPhrases.txt
```

和 gse 一樣, 使用 `-tags ne` 編譯可以不把詞典 (約 1.5 MB) 打包進執行檔;
設定仍然內建, 所以需要用 `LoadDict` 載入詞典:

```sh
go build -tags ne ./cmd/gocc && gocc -c s2t -path /usr/share/gocc/dictionary
```

`Simplified()`、`Traditional()` 和 `JP()` 回傳轉換器的輸出類型,
根據設定名稱判斷: `t2s` 為簡體, `s2twp` 為繁體, `t2jp` 為日文。

和 OpenCC 一樣, 設定中可以包含註解和結尾逗號。

`gocc.WithTofuRisk()` 會同時使用標記為 `may_output_tofu` 的詞典,
其中的罕用字在常見字型中可能無法顯示 (對應 OpenCC 的
`--include-tofu-risk-dictionaries`)。

命令列:

```sh
echo "不能踏入同一条河流" | gocc -c s2t
# 不能踏入同一條河流
gocc -c t2s -i in.txt -o out.txt
gocc -c my/config.json -path my/dicts  # 設定檔和詞典目錄
```

特殊輸入的處理: 無效的 UTF-8 位元組和 NUL 位元組會原樣輸出
(OpenCC 遇到無效的 UTF-8 會拋出例外)。`AppendConvert` 的 `dst`
不能和輸入重疊。

## 轉換

| 名稱                         | 轉換                                                 |
| ---------------------------- | ---------------------------------------------------- |
| `s2t`                        | 簡體中文到繁體中文                                   |
| `t2s`                        | 繁體中文到簡體中文                                   |
| `s2tw`                       | 簡體中文到繁體中文 (臺灣正體)                        |
| `tw2s`                       | 繁體中文 (臺灣正體) 到簡體中文                       |
| `s2hk`                       | 簡體中文到繁體中文 (香港繁體)                        |
| `hk2s`                       | 繁體中文 (香港繁體) 到簡體中文                       |
| `s2twp`                      | 簡體中文到繁體中文 (臺灣正體), 並轉換為臺灣常用詞彙  |
| `tw2sp`                      | 繁體中文 (臺灣正體) 到簡體中文, 並轉換為大陸常用詞彙 |
| `s2hkp`                      | 簡體中文到繁體中文 (香港繁體), 並轉換為香港常用詞彙  |
| `hk2sp`                      | 繁體中文 (香港繁體) 到簡體中文, 並轉換為大陸常用詞彙 |
| `t2tw`                       | 繁體中文 (OpenCC 標準) 到臺灣正體                    |
| `tw2t`                       | 繁體中文 (臺灣正體) 到繁體中文 (OpenCC 標準)         |
| `t2hk`                       | 繁體中文 (OpenCC 標準) 到香港繁體                    |
| `hk2t`                       | 繁體中文 (香港繁體) 到繁體中文 (OpenCC 標準)         |
| `t2jp`                       | 繁體中文漢字 (舊字體) 到日文新字體                   |
| `jp2t`                       | 日文新字體到繁體中文漢字 (舊字體)                    |
| `s2seal`, `t2seal`, `seal2t` | 篆書轉換                                             |

## 更新資料

`data/` 中是 OpenCC 的 `data/config/*.json` 和 `data/dictionary/*.txt`,
以及 OpenCC 在建置時產生的詞典 (`*Rev.txt`、
`TSCharactersExt.txt`、`STPhrases_GeneratedFromRegionalPhrases.txt`):

```sh
go run ./internal/cmd/gendict -opencc /path/to/OpenCC   # 同步並重新產生
go generate                                              # 只重新產生
```

## 程式碼檢查 (Lint)

CI 中使用 [golangci-lint](https://golangci-lint.run) v2 檢查程式碼,
設定見 [.golangci.yml](.golangci.yml):

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run ./...
golangci-lint run --build-tags ne ./...
```

## 授權條款

Apache-2.0。詞典和設定來自 OpenCC (Apache-2.0)。
