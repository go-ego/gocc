package gocc_test

import (
	"fmt"
	"os"
	"testing"

	gocc "github.com/go-ego/gocc"
)

func Example() {
	s2t, err := gocc.New("s2t")
	if err != nil {
		panic(err)
	}
	fmt.Println(s2t.Convert("自然语言处理是人工智能领域中的一个重要方向。"))

	tw := gocc.MustNew("s2twp")
	fmt.Println(tw.Convert("鼠标和内存"))
	// Output:
	// 自然語言處理是人工智能領域中的一個重要方向。
	// 滑鼠和記憶體
}

func BenchmarkConvert(b *testing.B) {
	in, err := os.ReadFile("testdata/golden/us_constitution_zhs.txt")
	if err != nil {
		b.Fatal(err)
	}
	for _, name := range []string{"s2t", "s2twp", "t2s"} {
		c := gocc.MustNew(name)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(in)))
			b.ReportAllocs()
			var dst []byte
			for b.Loop() {
				dst = c.AppendConvert(dst[:0], in)
			}
		})
	}
}

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		gocc.MustNew("s2twp")
	}
}
