package errcode

import (
	"encoding/json"
	"errors"
	"google.golang.org/grpc/status"
	"noy/router/pkg/yapr/logger"
	"strings"
)

type ErrWithCode struct {
	Code int               `json:"code"`
	Data map[string]string `json:"data,omitempty"`
	Msg  string            `json:"msg"`
}

func New(code int, msg string) *ErrWithCode {
	return &ErrWithCode{Code: code, Msg: msg}
}

func WithData(err *ErrWithCode, data map[string]string) *ErrWithCode {
	return &ErrWithCode{Code: err.Code, Data: data, Msg: err.Msg}
}

// Error implements the error interface.
func (e *ErrWithCode) Error() string {
	bytes, err := json.Marshal(e)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

func As(err error) (*ErrWithCode, bool) {
	for ; err != nil; err = errors.Unwrap(err) {
		s := err.Error()
		logger.Debugf("As: %s", s)
		if !strings.HasPrefix(s, "{") {
			continue
		}
		var e ErrWithCode
		if err := json.Unmarshal([]byte(s), &e); err != nil {
			continue
		}
		return &e, true
	}
	return nil, false
}

// Is implements the comparison for errors.Is.
func (e *ErrWithCode) Is(target error) bool {
	as, ok := As(target)
	if !ok {
		return false
	}
	return e.Code == as.Code
}

// ToGRPCError converts a custom error to a gRPC error with a status code.
func (e *ErrWithCode) ToGRPCError() error {
	st := status.New(10086, e.Error())
	return st.Err()
}

// FromGRPCError converts a gRPC error to a custom error if possible.
func FromGRPCError(err error) *ErrWithCode {
	st, ok := status.FromError(err)
	if !ok {
		return nil
	}
	var e ErrWithCode
	if err := json.Unmarshal([]byte(st.Message()), &e); err != nil {
		return nil
	}
	return &e
}

var (
	ErrContextCanceled     = New(1001, "context canceled")
	ErrRouteAlreadyExists  = New(1002, "route already exists")
	ErrRouterNotFound      = New(1003, "router not found")
	ErrSelectorNotFound    = New(1004, "selector not found")
	ErrServiceNotFound     = New(1005, "service not found")
	ErrNoEndpointAvailable = New(1006, "no endpoint available")
	ErrBadEndpoint         = New(1007, "bad endpoint")
	ErrNoRuleMatched       = New(1008, "no rule matched")
	ErrNoCustomRoute       = New(1009, "no custom route")
	ErrNoKeyAvailable      = New(1010, "no key available (for stateful routing)")
	ErrNoValueAvailable    = New(1011, "no value available (for stateful routing)")
	ErrBufferNotFound      = New(1012, "buffer not found")
	ErrBadDirectSelect     = New(1013, "bad direct select")
	ErrLuaScriptNotFound   = New(1014, "lua script not found")
	ErrLuaIndexOutOfRange  = New(1015, "lua index out of range")
	ErrBadLuaScript        = New(1016, "bad lua script")
	ErrWrongEndpoint       = New(1017, "wrong endpoint")
	ErrMetadataNotFound    = New(1018, "metadata not found")
	ErrInvalidMetadata     = New(1019, "invalid metadata")
	ErrMaxRetries          = New(1020, "max retries")
)
