package mcache

import (
	"bytes"
	"strconv"
	"testing"
)

func BenchmarkMcache_Get(b *testing.B) {
	c, err := NewMcache("hello.mcache")
	if err != nil {
		panic(err)
	}
	bbs := bytes.Repeat([]byte("hello"), 1)
	for i := 0; i < 100000; i++ {
		err = c.Set(strconv.Itoa(i), bbs)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb := c.Get("6000")
		if len(bb) == 0 {
			b.Fatal(i)
		}
	}
}
