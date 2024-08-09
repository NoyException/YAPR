package strategy

import (
	"google.golang.org/grpc/metadata"
	"hash/crc32"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/types"
)

func HashString(s string) uint32 {
	if len(s) < 64 {
		//声明一个数组长度为64
		var scratch [64]byte
		//拷贝数据到数组当中
		copy(scratch[:], s)
		//使用IEEE 多项式返回数据的CRC-32校验和   是一个标准 能帮助我们通过算法算出key对应的hash值
		return crc32.ChecksumIEEE(scratch[:len(s)])
	}
	return crc32.ChecksumIEEE([]byte(s))
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
