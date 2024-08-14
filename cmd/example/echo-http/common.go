package common

import "encoding/json"

type Response struct {
	Err     error             `json:"err"`
	Payload map[string]string `json:"payload"`
}

func (r *Response) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Response) Unmarshal(data []byte) error {
	return json.Unmarshal(data, r)
}
