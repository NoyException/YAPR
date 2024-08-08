package strategy

import (
	"google.golang.org/grpc/metadata"
	"hash/fnv"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
)

func HashString(s string) uint64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(s))
	if err != nil {
		logger.Errorf("hash string failed: %v", err)
		return 0
	}
	return h.Sum64()
}

func HeaderValue(headerKey string, match *types.MatchTarget) (string, error) {
	if headerKey == "" {
		return "", errcode.ErrNoKeyAvailable
	}

	md, exist := metadata.FromOutgoingContext(match.Ctx)
	if !exist {
		return "", errcode.ErrNoValueAvailable
	}
	values := md.Get(headerKey)
	if len(values) == 0 {
		return "", errcode.ErrNoValueAvailable
	}
	return values[0], nil
}
