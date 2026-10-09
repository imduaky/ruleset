package singjson

import "encoding/json/v2"

func UnmarshalStrict(data []byte, v any) error {
	return json.Unmarshal(data, v, json.RejectUnknownMembers(true))
}
