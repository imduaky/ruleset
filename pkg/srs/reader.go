// Modified from sing-box, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing-box/blob/fe92ab3e78a9bb7d448c155ef6906218e2ca5453/common/srs/binary.go

package srs

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/imduaky/ruleset/pkg/internal/singdomain"
	"github.com/imduaky/ruleset/pkg/internal/singvarbin"
	"github.com/imduaky/ruleset/pkg/srs/singjson"
)

// Read reads a rule-set the same as sing-box, and also verifies the zlib checksum.
// r should hold exactly one rule-set, since zlib may read past its end
// and the data after the zlib stream is not checked.
func Read(r io.Reader) (Ruleset, error) {
	version, err := readHeader(r)
	if err != nil {
		return Ruleset{}, err
	}
	compressReader, err := zlib.NewReader(r)
	if err != nil {
		return Ruleset{}, err
	}

	// zlib.Reader is not an io.ByteReader, which is required by uvarint.
	bReader := bufio.NewReader(compressReader)
	rules, err := readRules(bReader, 0)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Ruleset{}, io.ErrUnexpectedEOF
		}
		return Ruleset{}, err
	}
	// zlib verifies the checksum only when a Read reaches the end of its stream,
	// which may not happen with the last rule, so read once more for io.EOF.
	_, err = bReader.ReadByte()
	if err == nil {
		return Ruleset{}, errors.New("unexpected data after rules")
	}
	if err != io.EOF {
		return Ruleset{}, err
	}

	return Ruleset{Version: version, Rules: rules}, nil
}

func readHeader(r io.Reader) (RuleSetVersion, error) {
	var header [4]byte
	_, err := io.ReadFull(r, header[:])
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, io.ErrUnexpectedEOF
		}
		return 0, err
	}

	if [3]byte(header[:3]) != magicBytes {
		return 0, errors.New("not a srs")
	}

	version := RuleSetVersion(header[3])
	if !version.OK() {
		return 0, fmt.Errorf("invalid version: %d", version)
	}

	return version, nil
}

func readSingleRule(r singvarbin.Reader, depth int) (Rule, error) {
	ruleType, err := r.ReadByte()
	if err != nil {
		return Rule{}, err
	}
	var rule Rule
	switch ruleType {
	case 0:
		rule.Type = RuleTypeDefault
		rule.DefaultOptions, err = readDefaultRule(r)
	case 1:
		rule.Type = RuleTypeLogical
		rule.LogicalOptions, err = readLogicalRule(r, depth)
	default:
		err = fmt.Errorf("srs: unknown ruleType: %d", ruleType)
	}
	return rule, err
}

func readRules(r singvarbin.Reader, depth int) ([]Rule, error) {
	if depth > maxLogicalRuleDepth {
		return nil, errors.New("logical rule nesting is too deep")
	}
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	rules := make([]Rule, 0, min(length, 1024))
	for i := range length {
		rule, err := readSingleRule(r, depth)
		if err != nil {
			return nil, fmt.Errorf("read rule[%d]: %w", i, err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func readDefaultRule(r singvarbin.Reader) (DefaultRule, error) {
	var rule DefaultRule
	for {
		value, err := r.ReadByte()
		if err != nil {
			return DefaultRule{}, err
		}
		// Same as sing-box, item versions are only checked when writing.
		item := RuleItem(value)
		switch item {
		case RuleItemQueryType:
			rule.QueryType, err = singvarbin.ReadSlice[DNSQueryType](r)
		case RuleItemNetwork:
			rule.Network, err = readRuleItemString(r)
		case RuleItemDomain:
			rule.Domain, rule.DomainSuffix, err = singdomain.ReadMatcher(r)
		case RuleItemDomainKeyword:
			rule.DomainKeyword, err = readRuleItemString(r)
		case RuleItemDomainRegex:
			rule.DomainRegex, err = readRuleItemString(r)
		case RuleItemSourceIPCIDR:
			rule.SourceIPCIDR, err = readIPSet(r)
		case RuleItemIPCIDR:
			rule.IPCIDR, err = readIPSet(r)
		case RuleItemSourcePort:
			rule.SourcePort, err = singvarbin.ReadSlice[uint16](r)
		case RuleItemSourcePortRange:
			rule.SourcePortRange, err = readRuleItemString(r)
		case RuleItemPort:
			rule.Port, err = singvarbin.ReadSlice[uint16](r)
		case RuleItemPortRange:
			rule.PortRange, err = readRuleItemString(r)
		case RuleItemProcessName:
			rule.ProcessName, err = readRuleItemString(r)
		case RuleItemProcessPath:
			rule.ProcessPath, err = readRuleItemString(r)
		case RuleItemPackageName:
			rule.PackageName, err = readRuleItemString(r)
		case RuleItemWIFISSID:
			rule.WIFISSID, err = readRuleItemString(r)
		case RuleItemWIFIBSSID:
			rule.WIFIBSSID, err = readRuleItemString(r)
		case RuleItemAdGuardDomain:
			rule.AdGuardDomain, err = singdomain.ReadAdGuardMatcher(r)
		case RuleItemProcessPathRegex:
			rule.ProcessPathRegex, err = readRuleItemString(r)
		case RuleItemNetworkType:
			rule.NetworkType, err = singvarbin.ReadSlice[InterfaceType](r)
		case RuleItemNetworkIsExpensive:
			rule.NetworkIsExpensive = true
		case RuleItemNetworkIsConstrained:
			rule.NetworkIsConstrained = true
		case RuleItemNetworkInterfaceAddress:
			rule.NetworkInterfaceAddress, err = readInterfaceAddresses(r)
		case RuleItemDefaultInterfaceAddress:
			rule.DefaultInterfaceAddress, err = readPrefixes(r)
		case RuleItemPackageNameRegex:
			rule.PackageNameRegex, err = readRuleItemString(r)
		case RuleItem(RuleItemFinal):
			err = binary.Read(r, binary.BigEndian, &rule.Invert)
			return rule, err
		default:
			err = fmt.Errorf("unknown rule item: %d", item)
		}
		if err != nil {
			return DefaultRule{}, fmt.Errorf("read rule item[%d]: %w", item, err)
		}
	}
}

func readLogicalRule(r singvarbin.Reader, depth int) (LogicalRule, error) {
	value, err := r.ReadByte()
	if err != nil {
		return LogicalRule{}, err
	}
	rule := LogicalRule{Mode: LogicalRuleMode(value)}
	if !rule.Mode.OK() {
		return LogicalRule{}, fmt.Errorf("unknown logical mode: %d", value)
	}
	rule.Rules, err = readRules(r, depth+1)
	if err != nil {
		return LogicalRule{}, err
	}
	err = binary.Read(r, binary.BigEndian, &rule.Invert)
	return rule, err
}

func readRuleItemString(r singvarbin.Reader) ([]string, error) {
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, min(length, 1024))
	for range length {
		data, err := singvarbin.ReadSlice[byte](r)
		if err != nil {
			return nil, err
		}
		values = append(values, string(data))
	}
	return values, nil
}

func readInterfaceAddresses(r singvarbin.Reader) (*singjson.TypedMap[InterfaceType, singjson.Listable[singjson.Prefixable]], error) {
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	values := new(singjson.TypedMap[InterfaceType, singjson.Listable[singjson.Prefixable]])
	for range length {
		key, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		prefixes, err := readPrefixes(r)
		if err != nil {
			return nil, err
		}
		values.Put(InterfaceType(key), prefixes)
	}
	return values, nil
}
