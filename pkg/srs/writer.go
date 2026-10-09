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

const maxLogicalRuleDepth = 100

func Write(w io.Writer, ruleset Ruleset) error {
	if !ruleset.Version.OK() {
		return fmt.Errorf("invalid version: %d", ruleset.Version)
	}
	header := [4]byte{magicBytes[0], magicBytes[1], magicBytes[2], byte(ruleset.Version)}
	_, err := w.Write(header[:])
	if err != nil {
		return err
	}
	compressWriter, err := zlib.NewWriterLevel(w, zlib.BestCompression)
	if err != nil {
		return err
	}
	bWriter := bufio.NewWriter(compressWriter)
	err = writeRules(bWriter, ruleset.Rules, ruleset.Version, 0)
	if err != nil {
		return err
	}
	err = bWriter.Flush()
	if err != nil {
		return err
	}
	// Close writes the zlib checksum, it must be called exactly once.
	return compressWriter.Close()
}

func writeRules(w singvarbin.Writer, rules []Rule, version RuleSetVersion, depth int) error {
	if depth > maxLogicalRuleDepth {
		return errors.New("logical rule nesting is too deep")
	}
	err := singvarbin.WriteUvarint(w, uint64(len(rules)))
	if err != nil {
		return err
	}
	for i, rule := range rules {
		switch rule.Type {
		case RuleTypeDefault, "":
			err = writeDefaultRule(w, rule.DefaultOptions, version)
		case RuleTypeLogical:
			err = writeLogicalRule(w, rule.LogicalOptions, version, depth)
		default:
			err = fmt.Errorf("unknown rule type: %s", rule.Type)
		}
		if err != nil {
			return fmt.Errorf("write rule[%d]: %w", i, err)
		}
	}
	return nil
}

// writeDefaultRule writes items in the same order as sing-box,
// so the output is byte-for-byte identical.
func writeDefaultRule(w singvarbin.Writer, rule DefaultRule, version RuleSetVersion) error {
	err := w.WriteByte(0)
	if err != nil {
		return err
	}
	if len(rule.QueryType) > 0 {
		err = writeRuleItemSlice(w, RuleItemQueryType, rule.QueryType, version)
		if err != nil {
			return fmt.Errorf("query_type: %w", err)
		}
	}
	if len(rule.Network) > 0 {
		err = writeRuleItemString(w, RuleItemNetwork, rule.Network, version)
		if err != nil {
			return fmt.Errorf("network: %w", err)
		}
	}
	if len(rule.Domain) > 0 || len(rule.DomainSuffix) > 0 {
		err = writeRuleItemDomain(w, rule.Domain, rule.DomainSuffix, version)
		if err != nil {
			return fmt.Errorf("domain: %w", err)
		}
	}
	if len(rule.DomainKeyword) > 0 {
		err = writeRuleItemString(w, RuleItemDomainKeyword, rule.DomainKeyword, version)
		if err != nil {
			return fmt.Errorf("domain_keyword: %w", err)
		}
	}
	if len(rule.DomainRegex) > 0 {
		err = writeRuleItemString(w, RuleItemDomainRegex, rule.DomainRegex, version)
		if err != nil {
			return fmt.Errorf("domain_regex: %w", err)
		}
	}
	if len(rule.SourceIPCIDR) > 0 {
		err = writeRuleItemCIDR(w, RuleItemSourceIPCIDR, rule.SourceIPCIDR, version)
		if err != nil {
			return fmt.Errorf("source_ip_cidr: %w", err)
		}
	}
	if len(rule.IPCIDR) > 0 {
		err = writeRuleItemCIDR(w, RuleItemIPCIDR, rule.IPCIDR, version)
		if err != nil {
			return fmt.Errorf("ip_cidr: %w", err)
		}
	}
	if len(rule.SourcePort) > 0 {
		err = writeRuleItemSlice(w, RuleItemSourcePort, rule.SourcePort, version)
		if err != nil {
			return fmt.Errorf("source_port: %w", err)
		}
	}
	if len(rule.SourcePortRange) > 0 {
		err = writeRuleItemString(w, RuleItemSourcePortRange, rule.SourcePortRange, version)
		if err != nil {
			return fmt.Errorf("source_port_range: %w", err)
		}
	}
	if len(rule.Port) > 0 {
		err = writeRuleItemSlice(w, RuleItemPort, rule.Port, version)
		if err != nil {
			return fmt.Errorf("port: %w", err)
		}
	}
	if len(rule.PortRange) > 0 {
		err = writeRuleItemString(w, RuleItemPortRange, rule.PortRange, version)
		if err != nil {
			return fmt.Errorf("port_range: %w", err)
		}
	}
	if len(rule.ProcessName) > 0 {
		err = writeRuleItemString(w, RuleItemProcessName, rule.ProcessName, version)
		if err != nil {
			return fmt.Errorf("process_name: %w", err)
		}
	}
	if len(rule.ProcessPath) > 0 {
		err = writeRuleItemString(w, RuleItemProcessPath, rule.ProcessPath, version)
		if err != nil {
			return fmt.Errorf("process_path: %w", err)
		}
	}
	if len(rule.ProcessPathRegex) > 0 {
		err = writeRuleItemString(w, RuleItemProcessPathRegex, rule.ProcessPathRegex, version)
		if err != nil {
			return fmt.Errorf("process_path_regex: %w", err)
		}
	}
	if len(rule.PackageName) > 0 {
		err = writeRuleItemString(w, RuleItemPackageName, rule.PackageName, version)
		if err != nil {
			return fmt.Errorf("package_name: %w", err)
		}
	}
	if len(rule.PackageNameRegex) > 0 {
		err = writeRuleItemString(w, RuleItemPackageNameRegex, rule.PackageNameRegex, version)
		if err != nil {
			return fmt.Errorf("package_name_regex: %w", err)
		}
	}
	if len(rule.NetworkType) > 0 {
		err = writeRuleItemSlice(w, RuleItemNetworkType, rule.NetworkType, version)
		if err != nil {
			return fmt.Errorf("network_type: %w", err)
		}
	}
	if rule.NetworkIsExpensive {
		err = writeRuleItemType(w, RuleItemNetworkIsExpensive, version)
		if err != nil {
			return fmt.Errorf("network_is_expensive: %w", err)
		}
	}
	if rule.NetworkIsConstrained {
		err = writeRuleItemType(w, RuleItemNetworkIsConstrained, version)
		if err != nil {
			return fmt.Errorf("network_is_constrained: %w", err)
		}
	}
	if rule.NetworkInterfaceAddress != nil && rule.NetworkInterfaceAddress.Size() > 0 {
		err = writeRuleItemInterfaceAddress(w, rule.NetworkInterfaceAddress, version)
		if err != nil {
			return fmt.Errorf("network_interface_address: %w", err)
		}
	}
	if len(rule.DefaultInterfaceAddress) > 0 {
		err = writeRuleItemPrefixes(w, RuleItemDefaultInterfaceAddress, rule.DefaultInterfaceAddress, version)
		if err != nil {
			return fmt.Errorf("default_interface_address: %w", err)
		}
	}
	if len(rule.WIFISSID) > 0 {
		err = writeRuleItemString(w, RuleItemWIFISSID, rule.WIFISSID, version)
		if err != nil {
			return fmt.Errorf("wifi_ssid: %w", err)
		}
	}
	if len(rule.WIFIBSSID) > 0 {
		err = writeRuleItemString(w, RuleItemWIFIBSSID, rule.WIFIBSSID, version)
		if err != nil {
			return fmt.Errorf("wifi_bssid: %w", err)
		}
	}
	if len(rule.AdGuardDomain) > 0 {
		err = writeRuleItemAdGuardDomain(w, rule.AdGuardDomain, version)
		if err != nil {
			return fmt.Errorf("adguard_domain: %w", err)
		}
	}
	err = w.WriteByte(RuleItemFinal)
	if err != nil {
		return err
	}
	return binary.Write(w, binary.BigEndian, rule.Invert)
}

func writeLogicalRule(w singvarbin.Writer, rule LogicalRule, version RuleSetVersion, depth int) error {
	if !rule.Mode.OK() {
		return fmt.Errorf("unknown logical mode: %d", rule.Mode)
	}
	err := w.WriteByte(1)
	if err != nil {
		return err
	}
	err = w.WriteByte(byte(rule.Mode))
	if err != nil {
		return err
	}
	err = writeRules(w, rule.Rules, version, depth+1)
	if err != nil {
		return err
	}
	return binary.Write(w, binary.BigEndian, rule.Invert)
}

// writeRuleItemType checks the item is supported by version, then writes the item type.
func writeRuleItemType(w singvarbin.Writer, item RuleItem, version RuleSetVersion) error {
	var minimum RuleSetVersion
	switch item {
	case RuleItemAdGuardDomain:
		minimum = RuleSetVersion2
	case RuleItemNetworkType, RuleItemNetworkIsExpensive, RuleItemNetworkIsConstrained:
		minimum = RuleSetVersion3
	case RuleItemNetworkInterfaceAddress, RuleItemDefaultInterfaceAddress:
		minimum = RuleSetVersion4
	case RuleItemPackageNameRegex:
		minimum = RuleSetVersion5
	default:
		minimum = RuleSetVersion1
	}
	if version < minimum {
		return fmt.Errorf("requires SRS version %d or later", minimum)
	}
	return w.WriteByte(byte(item))
}

func writeRuleItemString(w singvarbin.Writer, item RuleItem, value []string, version RuleSetVersion) error {
	err := writeRuleItemType(w, item, version)
	if err != nil {
		return err
	}
	err = singvarbin.WriteUvarint(w, uint64(len(value)))
	if err != nil {
		return err
	}
	for _, value := range value {
		err = singvarbin.WriteString(w, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeRuleItemSlice[T ~uint8 | ~uint16](w singvarbin.Writer, item RuleItem, value []T, version RuleSetVersion) error {
	err := writeRuleItemType(w, item, version)
	if err != nil {
		return err
	}
	return singvarbin.WriteSlice(w, value)
}

func writeRuleItemDomain(w singvarbin.Writer, domain, domainSuffix []string, version RuleSetVersion) error {
	err := writeRuleItemType(w, RuleItemDomain, version)
	if err != nil {
		return err
	}
	return singdomain.WriteMatcher(w, domain, domainSuffix, version == RuleSetVersion1)
}

func writeRuleItemAdGuardDomain(w singvarbin.Writer, value []string, version RuleSetVersion) error {
	err := writeRuleItemType(w, RuleItemAdGuardDomain, version)
	if err != nil {
		return err
	}
	return singdomain.WriteAdGuardMatcher(w, value)
}

func writeRuleItemPrefixes(w singvarbin.Writer, item RuleItem, value []singjson.Prefixable, version RuleSetVersion) error {
	err := writeRuleItemType(w, item, version)
	if err != nil {
		return err
	}
	return writePrefixes(w, value)
}

func writeRuleItemInterfaceAddress(w singvarbin.Writer, value *singjson.TypedMap[InterfaceType, singjson.Listable[singjson.Prefixable]], version RuleSetVersion) error {
	err := writeRuleItemType(w, RuleItemNetworkInterfaceAddress, version)
	if err != nil {
		return err
	}
	err = singvarbin.WriteUvarint(w, uint64(value.Size()))
	if err != nil {
		return err
	}
	for _, entry := range value.Entries() {
		err = w.WriteByte(byte(entry.Key))
		if err != nil {
			return err
		}
		err = writePrefixes(w, entry.Value)
		if err != nil {
			return fmt.Errorf("%s: %w", entry.Key, err)
		}
	}
	return nil
}
