# gocc

[![Build Status](https://github.com/go-ego/gocc/workflows/Go/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/go.yml)
[![Lint Status](https://github.com/go-ego/gocc/workflows/Lint/badge.svg)](https://github.com/go-ego/gocc/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ego/gocc.svg)](https://pkg.go.dev/github.com/go-ego/gocc)

[English](https://github.com/go-ego/gocc/blob/main/README.md) | [繁體中文](https://github.com/go-ego/gocc/blob/main/README_zht.md) | [日本語](https://github.com/go-ego/gocc/blob/main/README_ja.md)

`gocc` 是 [OpenCC](https://github.com/BYVoid/OpenCC)
(Open Chinese Convert 开放中文转换) 的 Go 实现: 支持繁体中文、
简体中文和地区用字用词之间的转换。

遵循 OpenCC 1.4 的转换模型, 并内置其配置和词典,
词典用 [cedar](https://github.com/vcaesar/cedar) 双数组 trie 存储。

## 安装

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

自定义配置 (OpenCC JSON; `X.ocd2`/`X.ocd` 词典按 `X.txt` 读取):

```go
c, err := gocc.NewFromConfig(configJSON, os.DirFS("dicts"))
```

配置文件, 类似 OpenCC 的 `SimpleConverter`: 词典先在配置文件所在的目录中查找,
然后是 `LoadDict` 指定的目录, 最后是内置词典:

```go
c, err := gocc.NewFromFile("my/s2t_custom.json")
```

词典目录, 类似 OpenCC 的 `--path` 或 gse 的 `LoadDict`:
该目录下的文件会替换同名的内置词典:

```go
c, err := gocc.New("s2t", gocc.LoadDict("my/dicts")) // 例如 my/dicts/STPhrases.txt
```

和 gse 一样, 使用 `-tags ne` 编译可以不把词典 (约 1.5 MB) 打包进二进制;
配置仍然内置, 所以需要用 `LoadDict` 加载词典:

```sh
go build -tags ne ./cmd/gocc && gocc -c s2t -path /usr/share/gocc/dictionary
```

`Simplified()`、`Traditional()` 和 `JP()` 返回转换器的输出类型,
根据配置名判断: `t2s` 为简体, `s2twp` 为繁体, `t2jp` 为日文。

和 OpenCC 一样, 配置中可以包含注释和尾随逗号。

`gocc.WithTofuRisk()` 会同时使用标记为 `may_output_tofu` 的词典,
其中的生僻字在常见字体中可能无法显示 (对应 OpenCC 的
`--include-tofu-risk-dictionaries`)。

命令行:

```sh
echo "不能踏入同一条河流" | gocc -c s2t
# 不能踏入同一條河流
gocc -c t2s -i in.txt -o out.txt
gocc -c my/config.json -path my/dicts  # 配置文件和词典目录
```

特殊输入的处理: 无效的 UTF-8 字节和 NUL 字节会原样输出
(OpenCC 遇到无效的 UTF-8 会抛出异常)。`AppendConvert` 的 `dst`
不能和输入重叠。

## 转换

| 名称                         | 转换                                                 |
| ---------------------------- | ---------------------------------------------------- |
| `s2t`                        | 简体中文到繁体中文                                   |
| `t2s`                        | 繁体中文到简体中文                                   |
| `s2tw`                       | 简体中文到繁体中文 (台湾正体)                        |
| `tw2s`                       | 繁体中文 (台湾正体) 到简体中文                       |
| `s2hk`                       | 简体中文到繁体中文 (香港繁体)                        |
| `hk2s`                       | 繁体中文 (香港繁体) 到简体中文                       |
| `s2twp`                      | 简体中文到繁体中文 (台湾正体), 并转换为台湾常用词汇  |
| `tw2sp`                      | 繁体中文 (台湾正体) 到简体中文, 并转换为大陆常用词汇 |
| `s2hkp`                      | 简体中文到繁体中文 (香港繁体), 并转换为香港常用词汇  |
| `hk2sp`                      | 繁体中文 (香港繁体) 到简体中文, 并转换为大陆常用词汇 |
| `t2tw`                       | 繁体中文 (OpenCC 标准) 到台湾正体                    |
| `tw2t`                       | 繁体中文 (台湾正体) 到繁体中文 (OpenCC 标准)         |
| `t2hk`                       | 繁体中文 (OpenCC 标准) 到香港繁体                    |
| `hk2t`                       | 繁体中文 (香港繁体) 到繁体中文 (OpenCC 标准)         |
| `t2jp`                       | 繁体中文汉字 (旧字体) 到日文新字体                   |
| `jp2t`                       | 日文新字体到繁体中文汉字 (旧字体)                    |
| `s2seal`, `t2seal`, `seal2t` | 篆书转换                                             |

## 更新数据

`data/` 中是 OpenCC 的 `data/config/*.json` 和 `data/dictionary/*.txt`,
以及 OpenCC 在构建时生成的词典 (`*Rev.txt`、
`TSCharactersExt.txt`、`STPhrases_GeneratedFromRegionalPhrases.txt`):

```sh
go run ./internal/cmd/gendict -opencc /path/to/OpenCC   # 同步并重新生成
go generate                                              # 只重新生成
```

## 代码检查 (Lint)

CI 中使用 [golangci-lint](https://golangci-lint.run) v2 检查代码,
配置见 [.golangci.yml](.golangci.yml):

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run ./...
golangci-lint run --build-tags ne ./...
```

## 许可证

Apache-2.0。词典和配置来自 OpenCC (Apache-2.0)。
