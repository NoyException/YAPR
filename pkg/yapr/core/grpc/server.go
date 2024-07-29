package yapr

import "google.golang.org/grpc"

type Server struct {
	*grpc.Server

	service string
	ip      string
}

type ServerBuilder struct {
	service string
	ip      string
}

func NewServer() *ServerBuilder {
	return &ServerBuilder{}
}

func (s *ServerBuilder) WithService(service string) *ServerBuilder {
	s.service = service
	return s
}

func (s *ServerBuilder) WithIP(ip string) *ServerBuilder {
	s.ip = ip
	return s
}

func (s *ServerBuilder) Build() *Server {
	return &Server{
		service: s.service,
		ip:      s.ip,
	}
}
