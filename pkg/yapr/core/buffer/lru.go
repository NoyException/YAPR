package buffer

import (
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/store"
)

type node struct {
	HeaderValue string
	Endpoint    *types.Endpoint
	Prev        *node
	Next        *node
}

type LRUBuffer struct {
	selectorName string
	capacity     uint32
	buf          map[string]*node
	head         *node
	tail         *node
}

//var _ core.DynamicRouteBuffer = (*LRUBuffer)(nil)

func NewLRUBuffer(selectorName string, capacity uint32) *LRUBuffer {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}
	return &LRUBuffer{
		selectorName: selectorName,
		capacity:     capacity,
		buf:          make(map[string]*node),
	}
}

func (l *LRUBuffer) moveToHead(n *node) {
	// 如果是头结点，直接返回
	if l.head == n {
		return
	}
	// 先从链表中删除
	if n.Prev != nil {
		n.Prev.Next = n.Next
	}
	if n.Next != nil {
		n.Next.Prev = n.Prev
	}
	if l.tail == n {
		l.tail = n.Prev
	}
	// 将节点插入到头部
	n.Prev = nil
	n.Next = l.head
	if l.head != nil {
		l.head.Prev = n
	}
	l.head = n
	// 如果尾节点为空，说明链表为空，将尾节点指向头节点
	if l.tail == nil {
		l.tail = n
	}
}

func (l *LRUBuffer) resize() {
	for uint32(len(l.buf)) > l.capacity {
		delete(l.buf, l.tail.HeaderValue)
		l.tail = l.tail.Prev
		l.tail.Next = nil
	}
}

func (l *LRUBuffer) Get(headerValue string) *types.Endpoint {
	if n, ok := l.buf[headerValue]; ok {
		l.moveToHead(n)
		logger.Debugf("get from lru buffer, headerValue: %s, endpoint: %v", headerValue, n.Endpoint)
		return n.Endpoint
	}
	endpoint, err := store.MustStore().GetCustomRoute(l.selectorName, headerValue)
	if err != nil {
		return nil
	}
	n := &node{
		HeaderValue: headerValue,
		Endpoint:    endpoint,
	}
	l.buf[headerValue] = n
	l.moveToHead(n)
	l.resize()
	return endpoint
}
