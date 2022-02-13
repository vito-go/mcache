package mcache

import (
	"encoding/binary"
	"os"
	"sync"
	"syscall"
)

// 基于本地文件的一个缓存 基于MMAP技术

// Cache 缓存接口。
type Cache interface {
	// Set key value. The length of key must be less than 255,
	// and the length of value must be less than (4294967295-9) 3GB approximately.
	Set(key string, value []byte) error
	// Get the value of the key. It returns a nil []byte when key does not exist.
	Get(key string) []byte
	// Del the key.
	Del(key string)
}

// 设计要点 key长度小于255  单个value长度限制  255*255*255*255 大约3个GB
// 空间碎片的处理？
// 0. 单独的goroutine来处理
// 1. 碎片超过多大开始进行清理（或者占比多少？
// 2. 碎片清理 多少时间内 操作次数达到多少 开始清理
// 3. 清理碎片采用方式， 新的key插入一个新的映射地址中，然后遍历旧部的key插入地址中。
// 4. 碎片清理 旧的文件删除？

// 由于假定硬盘无限量，不考虑key的过期时间设置。 碎片处理先不实现，保持简单、

// 初始化机制： 暂且是先初始化个1<<20(不读取旧数据情况下)

// _count(4) _keyLen(1) KEY _valueLen(4) VALUE

type mcache struct {
	mu         sync.RWMutex // mu to protect the follow fields
	keyPos     map[string]uint32
	lastOffset uint32 // 扩容后的offset
	list       *doubleList
	offset     uint32 // 当前的offset
	f          *os.File
	byteAddr   []byte
}

// defaultLength 默认映射空间大小，不占用实际内存。
const defaultLength = 64 << 30

// firstLen 初始化空间大小
const firstLen = 64 << 20

// NewMcache create a new Mcache by file 。
// file is the path of the underlying file.
func NewMcache(file string) (Cache, error) {
	// TODO  os.O_CREATE|os.O_RDWR|os.O_APPEND 未来考虑持续化。
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	zero1024 := make([]byte, firstLen)
	_, err = f.Write(zero1024)
	if err != nil {
		return nil, err
	}
	// 追加用f.Write 读取和修改用MMap
	bytesAddr, err := syscall.Mmap(int(f.Fd()), 0, defaultLength, syscall.PROT_WRITE|syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	const keyCount = 1 << 10
	return &mcache{
		mu:         sync.RWMutex{},
		keyPos:     make(map[string]uint32, keyCount),
		lastOffset: firstLen,
		offset:     0,
		f:          f,
		list:       newLogList(),
		byteAddr:   bytesAddr,
	}, nil
}

func (m *mcache) Get(key string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	o, ok := m.keyPos[key]
	if ok {
		tCount := binary.BigEndian.Uint32(m.byteAddr[o : o+4])
		b := m.byteAddr[o : o+tCount]
		vStart := 4 + 1 + len(key) + 4 // value从开始count的位置
		return b[vStart:]
	}
	return nil
}

func (m *mcache) Del(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	o, ok := m.keyPos[key]
	if !ok {
		return
	}
	tCount := binary.BigEndian.Uint32(m.byteAddr[o : o+4])
	m.list.Insert(tCount, OffsetAB{A: o, B: o + tCount})
	delete(m.keyPos, key)
}

func (m *mcache) Set(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.keyPos[key]
	// 如果存在
	// OvCount := 1 + uint32(len(key)) + 4 + uint32(len(value)) // 总长不包含total4
	// 空间不够 先扩容
	tNowCount := 4 + 1 + uint32(len(key)) + 4 + uint32(len(value)) // 包含total4
	if ok {
		tCount := binary.BigEndian.Uint32(m.byteAddr[o : o+4])
		b := m.byteAddr[o : o+tCount]
		vStart := 4 + 1 + len(key)                              // value从开始count的位置
		vCount := binary.BigEndian.Uint32(b[vStart : vStart+4]) // value长度
		if uint32(len(value)) < vCount {
			copy(b[vStart+4:], value)
			binary.BigEndian.PutUint32(b[vStart:vStart+4], uint32(len(value))) // 更新vCount
			// binary.BigEndian.PutUint32(b[:4], tNowCount)                       // 更新tCount // todo 不更新可以吗？更新后碎片就多了
			// 更新总值
		} else if uint32(len(value)) == vCount {
			copy(b[vStart+4:], value)
			return nil
		} else {
			// 旧区间不够
			if m.lastOffset-m.offset < tNowCount {
				if err := m.grow(tNowCount); err != nil {
					return err
				}
			}
			// 空间容量够
			m.add(m.offset, key, value, false)

			m.offset += tNowCount
			m.list.Insert(tCount, OffsetAB{A: o, B: o + tCount})
			return nil
		}
	}
	// 先看看有、有没有空余的位置
	offsetAB, ok := m.list.Pop(tNowCount)
	if ok {
		m.add(offsetAB.A, key, value, true)
		return nil
	}
	// 空间不够 先扩容
	if m.lastOffset-m.offset < tNowCount {
		if err := m.grow(tNowCount); err != nil {
			return err
		}
	}
	// 空间容量够
	m.add(m.offset, key, value, true)
	m.offset += tNowCount
	return nil
}

// grow 写入操作采用先扩容再mmap write。 扩容机制：1.25倍+本次写入的大小
func (m *mcache) grow(tNowCount uint32) error {
	if _, err := m.f.Write(make([]byte, tNowCount+m.offset/4)); err != nil {
		return err
	}
	m.lastOffset += tNowCount + m.offset/4
	return nil
}

// add  起始的offset 容量够 纯增加 末尾曾
func (m *mcache) add(offset uint32, key string, value []byte, updateKey bool) {
	tCount := 4 + 1 + uint32(len(key)) + 4 + uint32(len(value)) // 包含 total4
	b := m.byteAddr[offset : tCount+offset]
	binary.BigEndian.PutUint32(b[:4], tCount) // 写入 tCount
	if updateKey {
		b[4] = byte(len(key))
		copy(b[5:], key)
	}
	binary.BigEndian.PutUint32(b[len(key)+1+4:len(key)+1+4+4], uint32(len(value)))
	copy(b[tCount-uint32(len(value)):], value)
	m.keyPos[key] = offset
}
