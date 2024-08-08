package types

type Service struct {
	Name           string                             `yaml:"name" json:"name,omitempty"`          // #唯一名称
	AttrMap        map[Endpoint]map[string]*Attribute `yaml:"-" json:"attr_map,omitempty"`         // *每个endpoint和他在某个 Selector 中的属性【etcd中保存为slt/$Name/AttrMap_$Endpoint -> $Attr】
	EndpointsByPod map[string][]*Endpoint             `yaml:"-" json:"endpoints_by_pod,omitempty"` // *每个pod对应的endpoints，【etcd不保存】

	dirty        bool                    // 是否需要更新endpoints
	endpoints    []*Endpoint             // 所有的endpoints
	endpointsSet map[Endpoint]struct{}   // 所有的endpoints
	attributes   []map[string]*Attribute // 所有的属性，idx和endpoints对应，selectorName -> attr

	UpdateNTF chan struct{} // *Service更新通知
}

func NewService(name string) *Service {
	return &Service{
		Name:           name,
		AttrMap:        make(map[Endpoint]map[string]*Attribute),
		EndpointsByPod: make(map[string][]*Endpoint),

		UpdateNTF: make(chan struct{}),
	}
}

func (s *Service) SetDirty() {
	s.dirty = true
}

func (s *Service) update() {
	endpoints := make([]*Endpoint, 0)
	endpointsSet := make(map[Endpoint]struct{})
	attributes := make([]map[string]*Attribute, 0)
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

func (s *Service) Attributes(selector string) []*Attribute {
	if s.dirty {
		s.update()
	}
	attrs := make([]*Attribute, 0)
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

func (s *Service) SetAttribute(endpoint *Endpoint, selector string, attr *Attribute) {
	m, ok := s.AttrMap[*endpoint]
	if !ok {
		m = make(map[string]*Attribute)
		s.AttrMap[*endpoint] = m
	}
	m[selector] = attr
	s.SetDirty()
}
