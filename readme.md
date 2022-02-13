# cache

- author: liushihao
- email: liushihao1993@hotmail.com 基于linux mmap技术的本地缓存文件存储、搜索系统，查询性能卓越，超百万QPS。

--- 

# Usage

```go
package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/vito-go/mcache"
)

func main() {
	c, err := mcache.NewMcache("hello.mcache")
	if err != nil {
		panic(err)
	}
	st := time.Now()
	for i := 0; i < 10000000; i++ {
		key := strconv.FormatInt(int64(i), 10)
		err = c.Set(key, []byte("hello world"+strconv.Itoa(i)))
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("写入用时：", time.Since(st))
	st = time.Now()
	for i := 0; i < 10000000; i++ {
		if string(c.Get(strconv.FormatInt(int64(i), 10))) != "hello world"+strconv.Itoa(i) {
			panic("not equal")
		}
	}
	fmt.Println("read用时：", time.Since(st))
}

```

## TODO 
碎片处理待实现