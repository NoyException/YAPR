package types

import (
	"noy/router/pkg/yapr/logger"
	"sync"
)

type Service struct {
	name           string                   // #唯一名称
	endpoints      []*Endpoint              // 所有的endpoints
	endpointsSet   map[Endpoint]struct{}    // 所有的endpoints
	endpointsByPod map[string][]*Endpoint   // *每个pod对应的endpoints
	attrMap        map[Endpoint]*Attributes // *每个endpoint和他的属性

	dirty bool // 是否需要更新endpoints

	mu      sync.RWMutex
	version uint64 // 版本号

	UpdateNTF chan struct{} // *Service更新通知
}

func NewService(name string) *Service {
	return &Service{
		name:           name,
		endpointsSet:   make(map[Endpoint]struct{}),
		endpointsByPod: make(map[string][]*Endpoint),
		attrMap:        make(map[Endpoint]*Attributes),

		mu: sync.RWMutex{},

		UpdateNTF: make(chan struct{}),
	}
}

func (s *Service) setDirty() {
	s.dirty = true
}

func (s *Service) update() {
	if !s.dirty {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dirty = false
	s.version++

	ntf := s.UpdateNTF
	s.UpdateNTF = make(chan struct{})
	close(ntf)
}

func (s *Service) Endpoints() []*Endpoint {
	if s.dirty {
		s.update()
	}
	return s.endpoints
}

func (s *Service) EndpointsSet() map[Endpoint]struct{} {
	if s.dirty {
		s.update()
	}
	return s.endpointsSet
}

func NewDefaultAttr() *AttributeInSelector {
	return &AttributeInSelector{
		Weight:   1,
		Deadline: 0,
	}
}

func (s *Service) GetAttribute(endpoint *Endpoint, selector string) *Attribute {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.attrMap[*endpoint]
	if !ok {
		return nil
	}
	return &Attribute{
		CommonAttribute:     m.CommonAttribute,
		AttributeInSelector: m.InSelector[selector],
	}
}

func (s *Service) Attributes(selector string) ([]*Endpoint, []*Attribute) {
	if s.dirty {
		s.update()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	attrs := make([]*Attribute, 0, len(s.endpoints))
	for _, a := range s.endpoints {
		attr, ok := s.attrMap[*a]
		if ok {
			inSelector, ok := attr.InSelector[selector]
			if !ok {
				inSelector = NewDefaultAttr()
			}
			attrs = append(attrs, &Attribute{
				CommonAttribute:     attr.CommonAttribute,
				AttributeInSelector: inSelector,
			})
		} else {
			attrs = append(attrs, &Attribute{
				CommonAttribute:     &CommonAttribute{Available: false},
				AttributeInSelector: NewDefaultAttr(),
			})
		}
	}
	return s.endpoints, attrs
}

// SetAttributeInSelector 设置endpoint的属性，需要调用方保证endpoint已经存在才能调用
func (s *Service) SetAttributeInSelector(endpoint *Endpoint, selector string, attr *AttributeInSelector) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.endpointsSet[*endpoint]
	if !ok {
		return
	}

	m, ok := s.attrMap[*endpoint]
	if !ok {
		m = &Attributes{
			CommonAttribute: &CommonAttribute{Available: true},
			InSelector:      make(map[string]*AttributeInSelector),
		}
		s.attrMap[*endpoint] = m
	}
	m.InSelector[selector] = attr
	s.setDirty()
}

func (s *Service) IsAvailable(endpoint *Endpoint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.attrMap[*endpoint]
	if !ok {
		return false
	}
	return m.Available
}

func (s *Service) Version() uint64 {
	if s.dirty {
		s.update()
	}
	return s.version
}

func (s *Service) RegisterPod(pod string, endpoints []*Endpoint) {
	defer s.update()
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldEndpoints, ok := s.endpointsByPod[pod]; ok {
		for _, endpoint := range oldEndpoints {
			delete(s.attrMap, *endpoint)
		}
	}

	for _, endpoint := range endpoints {
		s.attrMap[*endpoint] = &Attributes{
			CommonAttribute: &CommonAttribute{Available: true},
			InSelector:      make(map[string]*AttributeInSelector),
		}
		s.endpointsSet[*endpoint] = struct{}{}
		s.endpoints = append(s.endpoints, endpoint)
	}
	s.endpointsByPod[pod] = endpoints
	s.setDirty()
}

func (s *Service) RemovePod(pod string) {
	defer s.update()
	s.mu.Lock()
	defer s.mu.Unlock()

	endpoints := s.endpointsByPod[pod]
	for _, endpoint := range endpoints {
		logger.Debugf("delete endpoint: %v in service %v", endpoint, s.name)
		delete(s.attrMap, *endpoint)
		delete(s.endpointsSet, *endpoint)
	}
	s.endpoints = make([]*Endpoint, 0, len(s.endpointsSet))
	for endpoint := range s.endpointsSet {
		s.endpoints = append(s.endpoints, &endpoint)
	}
	delete(s.endpointsByPod, pod)
	s.setDirty()
}

func (s *Service) HangPod(pod string) {
	defer s.update()
	s.mu.Lock()
	defer s.mu.Unlock()

	endpoints := s.endpointsByPod[pod]
	for _, endpoint := range endpoints {
		if attr, ok := s.attrMap[*endpoint]; ok {
			attr.Available = false
		}
	}
	s.setDirty()
}
