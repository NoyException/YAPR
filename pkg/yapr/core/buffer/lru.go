package buffer

import (
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/store"
	"sync"
	"time"
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

	lazyRefreshPool map[string]struct{}
	lastRefreshTime int64

	mu sync.Mutex
}

var _ types.DynamicRouteBuffer = (*LRUBuffer)(nil)

func NewLRUBuffer(selectorName string, capacity uint32) *LRUBuffer {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}
	return &LRUBuffer{
		selectorName: selectorName,
		capacity:     capacity,
		buf:          make(map[string]*node),

		lazyRefreshPool: make(map[string]struct{}),
	}
}

func (l *LRUBuffer) remove(n *node) {
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

func (l *LRUBuffer) refresh(headerValue string) {
	if n, ok := l.buf[headerValue]; ok {
		var err error
		n.Endpoint, err = store.MustStore().GetCustomRoute(l.selectorName, headerValue)
		if err != nil {
			delete(l.buf, headerValue)
			l.remove(n)
		}
	}
}

func (l *LRUBuffer) moveToHead(n *node) {
	l.mu.Lock()
	defer l.mu.Unlock()
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

func (l *LRUBuffer) resize() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for uint32(len(l.buf)) > l.capacity {
		delete(l.buf, l.tail.HeaderValue)
		l.remove(l.tail)
	}
}

func (l *LRUBuffer) lazyRefresh(headerValue string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.lazyRefreshPool[headerValue]; ok {
		return
	}
	l.lazyRefreshPool[headerValue] = struct{}{}

	if time.Now().UnixMilli()-l.lastRefreshTime > 1000 {
		l.lastRefreshTime = time.Now().UnixMilli()
		go func() {
			l.mu.Lock()
			defer l.mu.Unlock()

			logger.Debugf("lazy refresh lru buffer")
			for k := range l.lazyRefreshPool {
				l.refresh(k)
			}
			l.lazyRefreshPool = make(map[string]struct{})
		}()
	}
}

func (l *LRUBuffer) Get(headerValue string) (*types.Endpoint, error) {
	if n, ok := l.buf[headerValue]; ok {
		l.moveToHead(n)
		l.lazyRefresh(headerValue)
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

//func (l *LRUBuffer) Set(headerValue string, endpoint *types.Endpoint, timeout int64) error {
//	if n, ok := l.buf[headerValue]; ok {
//		n.Endpoint = endpoint
//	}
//	return store.MustStore().SetCustomRoute(l.selectorName, headerValue, endpoint, timeout, false)
//}
//
//func (l *LRUBuffer) Remove(headerValue string) error {
//	if n, ok := l.buf[headerValue]; ok {
//		delete(l.buf, headerValue)
//		l.remove(n)
//	}
//	return store.MustStore().RemoveCustomRoute(l.selectorName, headerValue)
//}

func (l *LRUBuffer) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf = make(map[string]*node)
	l.head = nil
	l.tail = nil
}
