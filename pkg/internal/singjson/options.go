package singjson

import (
	"encoding/json/v2"
	"errors"
	"net/netip"
)

type Listable[T any] []T

func (l Listable[T]) MarshalJSON() ([]byte, error) {
	arrayList := []T(l)
	if len(arrayList) == 1 {
		return json.Marshal(arrayList[0])
	}
	return json.Marshal(arrayList)
}

func (l *Listable[T]) UnmarshalJSON(content []byte) error {
	var singleItem *T
	err := UnmarshalStrict(content, &singleItem)
	if err == nil {
		if singleItem != nil {
			*l = []T{*singleItem}
		}
		return nil
	}
	newErr := UnmarshalStrict(content, (*[]T)(l))
	if newErr == nil {
		return nil
	}

	return errors.Join(err, newErr)
}

type Prefixable netip.Prefix

func (p *Prefixable) MarshalJSON() ([]byte, error) {
	prefix := netip.Prefix(*p)
	if prefix.Bits() == prefix.Addr().BitLen() {
		return json.Marshal(prefix.Addr().String())
	}
	return json.Marshal(prefix.String())
}

func (p *Prefixable) UnmarshalJSON(content []byte) error {
	var value string
	err := json.Unmarshal(content, &value)
	if err != nil {
		return err
	}
	prefix, prefixErr := netip.ParsePrefix(value)
	if prefixErr == nil {
		*p = Prefixable(prefix)
		return nil
	}
	addr, addrErr := netip.ParseAddr(value)
	if addrErr == nil {
		*p = Prefixable(netip.PrefixFrom(addr, addr.BitLen()))
		return nil
	}
	return prefixErr
}
