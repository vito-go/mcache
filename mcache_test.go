package mcache

import (
	"bytes"
	"fmt"
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
func TestMcache_Del(t *testing.T) {
	d:=newLogList()
	d.Insert(1,OffsetAB{A: 1,B: 2})
	d.Insert(2,OffsetAB{A: 13,B: 32})
	d.Insert(3,OffsetAB{A: 1,B: 2})
	d.Insert(4,OffsetAB{A: 1,B: 2})
	fmt.Println(d.Pop(2))
	fmt.Println(d.Pop(2))
	fmt.Println(d.Pop(2))
	d.Insert(5,OffsetAB{A: 1,B: 2})

	fmt.Println(d.find(3))
	fmt.Println("---")

}