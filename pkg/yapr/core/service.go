package core

import "noy/router/pkg/yapr/core/types"

type Service struct {
	Name           string                                         `yaml:"name" json:"name,omitempty"`          // #唯一名称
	AttrMap        map[types.Endpoint]map[string]*types.Attribute `yaml:"-" json:"attr_map,omitempty"`         // *每个endpoint和他在某个 Selector 中的属性【etcd中保存为slt/$Name/AttrMap_$Endpoint -> $Attr】
	EndpointsByPod map[string][]*types.Endpoint                   `yaml:"-" json:"endpoints_by_pod,omitempty"` // *每个pod对应的endpoints，【etcd不保存】

	dirty        bool                          // 是否需要更新endpoints
	endpoints    []*types.Endpoint             // 所有的endpoints
	endpointsSet map[types.Endpoint]struct{}   // 所有的endpoints
	attributes   []map[string]*types.Attribute // 所有的属性，idx和endpoints对应，selectorName -> attr

	UpdateNTF chan struct{} // *Service更新通知

	//EndpointsAddNtf chan *Endpoint // *Endpoints更新通知
	//EndpointsDelNtf chan *Endpoint // *Endpoints删除通知
}

func NewService(name string) *Service {
	return &Service{
		Name:           name,
		AttrMap:        make(map[types.Endpoint]map[string]*types.Attribute),
		EndpointsByPod: make(map[string][]*types.Endpoint),

		UpdateNTF: make(chan struct{}),
		//EndpointsAddNtf: make(chan *core.Endpoint, 100),
		//EndpointsDelNtf: make(chan *core.Endpoint, 100),
	}
}

func (s *Service) SetDirty() {
	s.dirty = true
}

func (s *Service) update() {
	endpoints := make([]*types.Endpoint, 0)
	endpointsSet := make(map[types.Endpoint]struct{})
	attributes := make([]map[string]*types.Attribute, 0)
	for endpoint, attr := range s.AttrMap {
		endpoints = append(endpoints, &endpoint)
		endpointsSet[endpoint] = struct{}{}
		attributes = append(attributes, attr)
	}
	s.endpoints = endpoints
	s.endpointsSet = endpointsSet
	s.attributes = attributes
	s.dirty = false

	ntf := s.UpdateNTF
	s.UpdateNTF = make(chan struct{})
	close(ntf)
}

func (s *Service) Endpoints() []*types.Endpoint {
	if s.dirty {
		s.update()
	}
	return s.endpoints
}

func (s *Service) EndpointsSet() map[types.Endpoint]struct{} {
	if s.dirty {
		s.update()
	}
	return s.endpointsSet
}

func NewDefaultAttr() *types.Attribute {
	return &types.Attribute{
		Weight:   1,
		Deadline: 0,
	}
}

func (s *Service) Attributes(selector string) []*types.Attribute {
	if s.dirty {
		s.update()
	}
	attrs := make([]*types.Attribute, 0)
	for _, attrMap := range s.attributes {
		if attr, ok := attrMap[selector]; ok {
			attrs = append(attrs, attr)
		} else {
			attr := NewDefaultAttr()
			attrMap[selector] = attr
			attrs = append(attrs, attr)
		}
	}
	return attrs
}

func (s *Service) SetAttribute(endpoint *types.Endpoint, selector string, attr *types.Attribute) {
	m, ok := s.AttrMap[*endpoint]
	if !ok {
		m = make(map[string]*types.Attribute)
		s.AttrMap[*endpoint] = m
	}
	m[selector] = attr
	s.SetDirty()
}
