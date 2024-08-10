package types

import (
	"noy/router/pkg/yapr/logger"
	"sync"
)

type Service struct {
	Name           string                   `yaml:"name" json:"name,omitempty"`          // #唯一名称
	AttrMap        map[Endpoint]*Attributes `yaml:"-" json:"attr_map,omitempty"`         // *每个endpoint和他的属性
	EndpointsByPod map[string][]*Endpoint   `yaml:"-" json:"endpoints_by_pod,omitempty"` // *每个pod对应的endpoints

	dirty        bool                  // 是否需要更新endpoints
	endpoints    []*Endpoint           // 所有的endpoints
	endpointsSet map[Endpoint]struct{} // 所有的endpoints
	attributes   []*Attributes         // 所有的属性，idx和endpoints对应

	mu      sync.RWMutex
	version uint64 // 版本号

	UpdateNTF chan struct{} // *Service更新通知
}

func NewService(name string) *Service {
	return &Service{
		Name:           name,
		AttrMap:        make(map[Endpoint]*Attributes),
		EndpointsByPod: make(map[string][]*Endpoint),

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

	endpoints := make([]*Endpoint, 0)
	endpointsSet := make(map[Endpoint]struct{})
	attributes := make([]*Attributes, 0)
	for endpoint, attr := range s.AttrMap {
		endpoints = append(endpoints, &endpoint)
		endpointsSet[endpoint] = struct{}{}
		attributes = append(attributes, attr)
	}
	s.endpoints = endpoints
	s.endpointsSet = endpointsSet
	s.attributes = attributes
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

func NewDefaultAttr() *Attribute {
	return &Attribute{
		Weight:   1,
		Deadline: 0,
	}
}

func (s *Service) AttributesInSelector(selector string) ([]*Endpoint, []*Attribute) {
	if s.dirty {
		s.update()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	attrs := make([]*Attribute, 0)
	for _, attrMap := range s.attributes {
		if attr, ok := attrMap.InSelector[selector]; ok {
			attrs = append(attrs, attr)
		} else {
			attr := NewDefaultAttr()
			attrMap.InSelector[selector] = attr
			attrs = append(attrs, attr)
		}
	}
	return s.endpoints, attrs
}

// SetAttribute 设置endpoint的属性，需要调用方保证endpoint已经存在才能调用
func (s *Service) SetAttribute(endpoint *Endpoint, selector string, attr *Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.AttrMap[*endpoint]
	if !ok {
		m = &Attributes{
			Available:  true,
			InSelector: make(map[string]*Attribute),
		}
		s.AttrMap[*endpoint] = m
	}
	m.InSelector[selector] = attr
	s.setDirty()
}

func (s *Service) IsAvailable(endpoint *Endpoint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.AttrMap[*endpoint]
	if !ok {
		return false
	}
	return m.Available
}

func (s *Service) GetAttribute(endpoint *Endpoint, selector string) *Attribute {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.AttrMap[*endpoint]
	if !ok {
		return nil
	}
	return m.InSelector[selector]
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

	if oldEndpoints, ok := s.EndpointsByPod[pod]; ok {
		for _, endpoint := range oldEndpoints {
			delete(s.AttrMap, *endpoint)
		}
	}

	for _, endpoint := range endpoints {
		s.AttrMap[*endpoint] = &Attributes{
			Available:  true,
			InSelector: make(map[string]*Attribute),
		}
	}
	s.EndpointsByPod[pod] = endpoints
	s.setDirty()
}

func (s *Service) RemovePod(pod string) {
	defer s.update()
	s.mu.Lock()
	defer s.mu.Unlock()

	endpoints := s.EndpointsByPod[pod]
	for _, endpoint := range endpoints {
		logger.Debugf("delete endpoint: %v in service %v", endpoint, s.Name)
		delete(s.AttrMap, *endpoint)
	}
	delete(s.EndpointsByPod, pod)
	s.setDirty()
}

func (s *Service) HangPod(pod string) {
	defer s.update()
	s.mu.Lock()
	defer s.mu.Unlock()

	endpoints := s.EndpointsByPod[pod]
	for _, endpoint := range endpoints {
		if attr, ok := s.AttrMap[*endpoint]; ok {
			attr.Available = false
		}
	}
	s.setDirty()
}
