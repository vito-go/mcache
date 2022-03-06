package mcache

import (
	"sync"
)

type Node struct {
	bLen      uint32     // 长度
	offsetABs []OffsetAB // 解析的时候不排序 展示的时候排序。 查询的频率也不高；入队列的时候直接是有序的
	prev      *Node      // 判断第一个节点 nil
	next      *Node      // 判断最后一个节点 nil
}
type OffsetAB struct {
	A uint32
	B uint32
}

// doubleList  双端链表 并发安全
type doubleList struct {
	mux    sync.RWMutex // 保护以下字段
	first  *Node
	last   *Node
	length int
	cap    int //  cap 容量 小于=0 则不限制容量
}

// newLogList 新建一个 doubleList .cap 限定logList的容量. 小于等于0则不限制
func newLogList() *doubleList {
	return &doubleList{mux: sync.RWMutex{}, cap: 0}
}

// Insert 节点新增一个OffsetAB.
func (d *doubleList) Insert(bLen uint32, ab OffsetAB) {
	d.mux.Lock()
	defer d.mux.Unlock()
	if element, ok := d.find(bLen); ok {
		element.offsetABs = append(element.offsetABs, ab)
		return
	}
	defer func() {
		if d.cap > 0 && d.length > d.cap {
			if first := d.first; first != nil {
				next := first.next
				if next != nil {
					first = nil
					next.prev = nil
					d.first = next
					d.length--
				}
			}
		}
	}()
	nd := Node{
		bLen:      bLen,
		offsetABs: []OffsetAB{ab},
		prev:      nil,
		next:      nil,
	}
	// 没有节点 等同于 d.Length==0
	if d.length == 0 {
		d.first = &nd
		d.last = &nd
		d.length++
		return
	}
	switch d.length {
	case 0:
		d.first = &nd
		d.last = &nd
		d.length++
		return
	case 1:
		switch {
		case bLen > d.last.bLen:
			d.last.next = &nd
			nd.prev = d.last
			d.last = &nd
			d.length++
			return
		case bLen < d.last.bLen:
			d.last.prev = &nd
			nd.next = d.last
			d.first = &nd
			d.length++
			return
		}
	}
	if bLen > d.last.bLen {
		d.last.next = &nd
		nd.prev = d.last
		d.last = &nd
		d.length++
		return
	}
	if bLen < d.first.bLen {
		d.first.prev = &nd
		nd.next = d.first
		d.first = &nd
		d.length++
		return
	}
	preNode := d.last.prev
	for bLen < preNode.bLen {
		preNode = preNode.prev
	}
	next := preNode.next
	nd.prev = preNode
	nd.next = next
	preNode.next = &nd
	next.prev = &nd
	d.length++
	return
}

// Find 通过tid查找节点 返回最小的节点. true,存在, false 节点不存在.
func (d *doubleList) Find(bLen uint32) (*Node, bool) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	return d.find(bLen)
}
func (d *doubleList) find(bLen uint32) (*Node, bool) {

	element := d.first
	if element == nil {
		return nil, false
	}
	for element != nil {
		// TODO dead loop??
		if element.bLen == bLen {
			return element, true
		}
		element = element.next
	}
	return nil, false
}

// ele must be a node of a.
func (d *doubleList) del(ele *Node) {
	if d.length == 0 {
		return
	}
	if d.length == 1 {
		d.length = 0
		d.last = nil
		d.first = nil
		return
	}
	pre := ele.prev
	next := ele.prev
	if pre != nil {
		pre.next = next
	}
	if next != nil {
		next.prev = pre
	}
	d.length--
	return
}

// Pop 通过tid查找节点 返回最小的节点. true,存在, false 节点不存在.
func (d *doubleList) Pop(bLen uint32) (OffsetAB, bool) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	element := d.first
	if element == nil {
		return OffsetAB{}, false
	}
	for element != nil {
		if element.bLen >= bLen {
			// 理论上这里不可能为0
			offsetAB := element.offsetABs[0]
			if len(element.offsetABs) == 1 {
				d.del(element)
				return offsetAB, true
			}
			element.offsetABs = element.offsetABs[1:]
			return offsetAB, true
		}
		element = element.next
	}
	return OffsetAB{}, false
}

func (d *doubleList) Length() int {
	if d == nil {
		return 0
	}
	return d.length
}

func (d *doubleList) Cap() int {
	if d == nil {
		return 0
	}
	return d.cap
}

func (n *Node) Tid() uint32 {
	if n == nil {
		return 0
	}
	return n.bLen
}

func (n *Node) OffsetABs() []OffsetAB {
	if n == nil {
		return nil
	}
	return n.offsetABs
}
