// Modified from sing-box, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing-box/blob/fe92ab3e78a9bb7d448c155ef6906218e2ca5453/common/srs/ip_cidr.go
// https://github.com/SagerNet/sing-box/blob/fe92ab3e78a9bb7d448c155ef6906218e2ca5453/common/srs/ip_set.go

package srs

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"

	"github.com/imduaky/ruleset/pkg/internal/singvarbin"
	"github.com/imduaky/ruleset/pkg/srs/singjson"

	"go4.org/netipx"
)

const (
	ipv4Length = 4
	ipv6Length = 16
)

func readAddress(r singvarbin.Reader) (netip.Addr, error) {
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return netip.Addr{}, err
	}

	if length != ipv4Length && length != ipv6Length {
		return netip.Addr{}, fmt.Errorf("invalid IP address length: %d", length)
	}

	var data = make([]byte, length)
	_, err = io.ReadFull(r, data[:length])
	if err != nil {
		return netip.Addr{}, err
	}
	addr, _ := netip.AddrFromSlice(data[:length])
	return addr, nil
}

func writeAddress(w singvarbin.Writer, addr netip.Addr) error {
	if !addr.IsValid() || addr.Zone() != "" {
		return fmt.Errorf("invalid IP address: %s", addr)
	}

	return singvarbin.WriteSlice(w, addr.AsSlice())
}

func readPrefix(r singvarbin.Reader) (singjson.Prefixable, error) {
	addr, err := readAddress(r)
	if err != nil {
		return singjson.Prefixable{}, err
	}
	bits, err := r.ReadByte()
	if err != nil {
		return singjson.Prefixable{}, err
	}
	prefix := netip.PrefixFrom(addr, int(bits))
	if !prefix.IsValid() {
		return singjson.Prefixable{}, fmt.Errorf("invalid prefix length: %d", bits)
	}
	return singjson.Prefixable(prefix), nil
}

func readPrefixes(r singvarbin.Reader) ([]singjson.Prefixable, error) {
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}

	values := make([]singjson.Prefixable, 0, length)
	for i := range length {
		prefix, err := readPrefix(r)
		if err != nil {
			return nil, fmt.Errorf("prefix[%d]: %w", i, err)
		}
		values = append(values, prefix)
	}
	return values, nil
}

func writePrefixes(w singvarbin.Writer, prefixes []singjson.Prefixable) error {
	err := singvarbin.WriteUvarint(w, uint64(len(prefixes)))
	if err != nil {
		return err
	}
	for i, value := range prefixes {
		prefix := netip.Prefix(value)
		if !prefix.IsValid() {
			return fmt.Errorf("invalid prefix[%d]: %s", i, prefix)
		}
		err = writeAddress(w, prefix.Addr())
		if err != nil {
			return err
		}
		err = w.WriteByte(byte(prefix.Bits()))
		if err != nil {
			return err
		}
	}
	return nil
}

func readIPSet(r singvarbin.Reader) ([]singjson.Prefixable, error) {
	version, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != 1 {
		return nil, fmt.Errorf("invalid IP set version: %d", version)
	}
	var length uint64
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}
	var builder netipx.IPSetBuilder
	for i := uint64(0); i < length; i++ {
		from, err := readAddress(r)
		if err != nil {
			return nil, err
		}
		to, err := readAddress(r)
		if err != nil {
			return nil, err
		}
		value := netipx.IPRangeFrom(from, to)
		if !value.IsValid() {
			return nil, fmt.Errorf("invalid IP range[%d]: %s - %s", i, from, to)
		}
		builder.AddRange(value)
	}
	set, err := builder.IPSet()
	if err != nil {
		return nil, err
	}
	prefixes := set.Prefixes()
	result := make([]singjson.Prefixable, len(prefixes))
	for i, prefix := range prefixes {
		result[i] = singjson.Prefixable(prefix)
	}
	return result, nil
}

func writeRuleItemCIDR(w singvarbin.Writer, item RuleItem, prefixes []singjson.Prefixable, version RuleSetVersion) error {
	var builder netipx.IPSetBuilder
	for i, value := range prefixes {
		prefix := netip.Prefix(value)
		if !prefix.IsValid() {
			return fmt.Errorf("invalid prefix[%d]: %s", i, prefix)
		}
		builder.AddPrefix(prefix)
	}
	set, err := builder.IPSet()
	if err != nil {
		return err
	}
	err = writeRuleItemType(w, item, version)
	if err != nil {
		return err
	}
	err = w.WriteByte(1)
	if err != nil {
		return err
	}
	ranges := set.Ranges()
	err = binary.Write(w, binary.BigEndian, uint64(len(ranges)))
	if err != nil {
		return err
	}

	// In sing-box native implementation , here will put ipv4 first and put ipv6 later.
	// While, the read function don't care about the order, so just put the ip address mixed
	// ipv4 and ipv6 is fine.
	for _, value := range ranges {
		err = writeAddress(w, value.From())
		if err != nil {
			return err
		}
		err = writeAddress(w, value.To())
		if err != nil {
			return err
		}
	}
	return nil
}
