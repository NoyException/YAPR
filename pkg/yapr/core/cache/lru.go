package cache

import (
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"sync"
)

type node struct {
	HeaderValue string
	Endpoint    *types.Endpoint
	Prev        *node
	Next        *node
}

type LRUCache struct {
	selectorName string
	capacity     uint32
	buf          map[string]*node
	head         *node
	tail         *node

	mu sync.Mutex
}

var _ types.DirectCache = (*LRUCache)(nil)

func NewLRUBuffer(selectorName string, capacity uint32) *LRUCache {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}
	return &LRUCache{
		selectorName: selectorName,
		capacity:     capacity,
		buf:          make(map[string]*node),
	}
}

func (l *LRUCache) remove(n *node) {
	if n.Prev != nil {
		n.Prev.Next = n.Next
	}
	if n.Next != nil {
		n.Next.Prev = n.Prev
	}
	if l.head == n {
		l.head = n.Next
	}
	if l.tail == n {
		l.tail = n.Prev
	}
}

func (l *LRUCache) moveToHead(n *node) {
	// 如果是头结点，直接返回
	if l.head == n {
		return
	}
	// 先从链表中删除
	l.remove(n)
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

func (l *LRUCache) resize() {
	for uint32(len(l.buf)) > l.capacity {
		delete(l.buf, l.tail.HeaderValue)
		l.remove(l.tail)
	}
}

func (l *LRUCache) Get(headerValue string) (*types.Endpoint, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n, ok := l.buf[headerValue]; ok {
		l.moveToHead(n)
		logger.Debugf("get from lru buffer, headerValue: %s, endpoint: %v", headerValue, n.Endpoint)
		return n.Endpoint, nil
	}
	endpoint, err := store.MustStore().GetCustomRoute(l.selectorName, headerValue)
	if err != nil {
		return nil, err
	}
	n := &node{
		HeaderValue: headerValue,
		Endpoint:    endpoint,
	}
	l.buf[headerValue] = n
	l.moveToHead(n)
	l.resize()
	return endpoint, nil
}

func (l *LRUCache) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf = make(map[string]*node)
	l.head = nil
	l.tail = nil
}

func (l *LRUCache) Refresh(headerValue string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n, ok := l.buf[headerValue]; ok {
		var err error
		n.Endpoint, err = store.MustStore().GetCustomRoute(l.selectorName, headerValue)
		if err != nil {
			delete(l.buf, headerValue)
			l.remove(n)
		}
		logger.Debugf("refresh lru buffer, headerValue: %s, endpoint: %v", headerValue, n.Endpoint)
	}
}
