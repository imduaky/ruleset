// Modified from sing, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/json/badjson/typed.go

package singjson

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

// TypedMap is a JSON object which keeps the order of entries, K must be encoded as a JSON string.
// It contains a linked list, so use it by pointer, same as sing-box.
type TypedMap[K comparable, V any] struct {
	linkedHashMap[K, V]
}

func (m *TypedMap[K, V]) MarshalJSONTo(encoder *jsontext.Encoder) error {
	err := encoder.WriteToken(jsontext.BeginObject)
	if err != nil {
		return err
	}
	for _, entry := range m.Entries() {
		err = json.MarshalEncode(encoder, entry.Key)
		if err != nil {
			return err
		}
		err = json.MarshalEncode(encoder, entry.Value)
		if err != nil {
			return err
		}
	}
	return encoder.WriteToken(jsontext.EndObject)
}

// UnmarshalJSONFrom decodes into a temporary map, m is only updated on success.
// The map can not be copied by value, so it is updated by Clear and PutAll which never fail.
func (m *TypedMap[K, V]) UnmarshalJSONFrom(decoder *jsontext.Decoder) error {
	token, err := decoder.ReadToken()
	if err != nil {
		return err
	}
	switch token.Kind() {
	case 'n':
		m.Clear()
		return nil
	case '{':
	default:
		return fmt.Errorf("expected json object, but got %s", token.Kind())
	}
	result := new(TypedMap[K, V])
	for decoder.PeekKind() != '}' {
		var key K
		err = json.UnmarshalDecode(decoder, &key)
		if err != nil {
			return err
		}
		var value V
		err = json.UnmarshalDecode(decoder, &value)
		if err != nil {
			return err
		}
		result.Put(key, value)
	}
	_, err = decoder.ReadToken()
	if err != nil {
		return err
	}
	m.Clear()
	m.PutAll(result)
	return nil
}
