package singjson

import (
	"bytes"
	"encoding/json"
)

func UnmarshalStrict(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	return decoder.Decode(v)
}
