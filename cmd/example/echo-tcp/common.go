package echo_tcp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"noy/router/pkg/yapr/core/errcode"
)

type Data struct {
	Headers map[string]string `json:"headers"`
	Data    []byte            `json:"data"`
	Err     error             `json:"err"`
}

func (d *Data) Marshal() []byte {
	var buf bytes.Buffer

	// Encode Headers
	headerCount := int32(len(d.Headers))
	binary.Write(&buf, binary.LittleEndian, headerCount)
	for key, value := range d.Headers {
		keyLen := int32(len(key))
		valueLen := int32(len(value))
		binary.Write(&buf, binary.LittleEndian, keyLen)
		buf.WriteString(key)
		binary.Write(&buf, binary.LittleEndian, valueLen)
		buf.WriteString(value)
	}

	// Encode Data
	dataLen := int32(len(d.Data))
	binary.Write(&buf, binary.LittleEndian, dataLen)
	buf.Write(d.Data)

	// Encode Err
	if d.Err != nil {
		errMsg := d.Err.Error()
		errLen := int32(len(errMsg))
		binary.Write(&buf, binary.LittleEndian, errLen)
		buf.WriteString(errMsg)
	} else {
		binary.Write(&buf, binary.LittleEndian, int32(0))
	}

	return buf.Bytes()
}

func UnmarshalData(b []byte) *Data {
	buf := bytes.NewReader(b)
	d := &Data{Headers: make(map[string]string)}

	// Decode Headers
	var headerCount int32
	binary.Read(buf, binary.LittleEndian, &headerCount)
	for i := int32(0); i < headerCount; i++ {
		var keyLen, valueLen int32
		binary.Read(buf, binary.LittleEndian, &keyLen)
		key := make([]byte, keyLen)
		buf.Read(key)
		binary.Read(buf, binary.LittleEndian, &valueLen)
		value := make([]byte, valueLen)
		buf.Read(value)
		d.Headers[string(key)] = string(value)
	}

	// Decode Data
	var dataLen int32
	binary.Read(buf, binary.LittleEndian, &dataLen)
	d.Data = make([]byte, dataLen)
	buf.Read(d.Data)

	// Decode Err
	var errLen int32
	binary.Read(buf, binary.LittleEndian, &errLen)
	if errLen > 0 {
		errMsg := make([]byte, errLen)
		buf.Read(errMsg)
		errWithCode := errcode.UnmarshalError(string(errMsg))
		if errWithCode != nil {
			d.Err = errWithCode
		} else {
			d.Err = fmt.Errorf(string(errMsg))
		}
	}

	return d
}
