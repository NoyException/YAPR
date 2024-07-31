package core

func NewService(name string) *Service {
	return &Service{
		Name:           name,
		AttrMap:        make(map[Endpoint]map[string]*Attribute),
		EndpointsByPod: make(map[string][]*Endpoint),
		//EndpointsAddNtf: make(chan *core.Endpoint, 100),
		//EndpointsDelNtf: make(chan *core.Endpoint, 100),
	}
}

func (s *Service) SetDirty() {
	s.dirty = true
}

func (s *Service) update() {
	endpoints := make([]*Endpoint, 0)
	attributes := make([]map[string]*Attribute, 0)
	for endpoint, attr := range s.AttrMap {
		endpoints = append(endpoints, &endpoint)
		attributes = append(attributes, attr)
	}
	s.endpoints = endpoints
	s.attributes = attributes
	s.dirty = false
}

func (s *Service) Endpoints() []*Endpoint {
	if s.dirty {
		s.update()
	}
	return s.endpoints
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
