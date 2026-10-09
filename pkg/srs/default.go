package srs

import (
	"encoding/json/v2"
	"fmt"
	"strconv"
	"strings"

	"github.com/imduaky/ruleset/pkg/internal/common"
	"github.com/imduaky/ruleset/pkg/internal/singdns"
)

var magicBytes = [3]byte{0x53, 0x52, 0x53} // SRS

// RuleItem codes are the same as sing-box, by nekohasekai <contact-sagernet@sekai.icu>.
// https://github.com/SagerNet/sing-box/blob/fe92ab3e78a9bb7d448c155ef6906218e2ca5453/common/srs/binary.go
type RuleItem uint8

const (
	RuleItemQueryType RuleItem = iota
	RuleItemNetwork
	RuleItemDomain
	RuleItemDomainKeyword
	RuleItemDomainRegex
	RuleItemSourceIPCIDR
	RuleItemIPCIDR
	RuleItemSourcePort
	RuleItemSourcePortRange
	RuleItemPort
	RuleItemPortRange
	RuleItemProcessName
	RuleItemProcessPath
	RuleItemPackageName
	RuleItemWIFISSID
	RuleItemWIFIBSSID
	RuleItemAdGuardDomain
	RuleItemProcessPathRegex
	RuleItemNetworkType
	RuleItemNetworkIsExpensive
	RuleItemNetworkIsConstrained
	RuleItemNetworkInterfaceAddress
	RuleItemDefaultInterfaceAddress
	RuleItemPackageNameRegex
	RuleItemFinal uint8 = 0xFF
)

type InterfaceType uint8

const (
	InterfaceTypeWIFI InterfaceType = iota
	InterfaceTypeCellular
	InterfaceTypeEthernet
	InterfaceTypeOther
)

func (o InterfaceType) String() string {
	so, exist := interfaceTypeToString[o]
	if exist {
		return so
	}
	return fmt.Sprintf("InterfaceType(%d)", o)
}

func ParseInterfaceType(s string) (InterfaceType, bool) {
	so, exist := stringToInterfaceType[strings.ToLower(s)]
	return so, exist
}

func (o InterfaceType) MarshalText() ([]byte, error) {
	value, ok := interfaceTypeToString[o]
	if !ok {
		return nil, fmt.Errorf("marshalText: invalid InterfaceType: %d", o)
	}
	return []byte(value), nil
}

func (o *InterfaceType) UnmarshalText(data []byte) error {
	value, ok := ParseInterfaceType(string(data))
	if !ok {
		return fmt.Errorf("unmarshalText: invalid InterfaceType: %s", data)
	}
	*o = value
	return nil
}

var (
	interfaceTypeToString = map[InterfaceType]string{
		InterfaceTypeWIFI:     "wifi",
		InterfaceTypeCellular: "cellular",
		InterfaceTypeEthernet: "ethernet",
		InterfaceTypeOther:    "other",
	}

	stringToInterfaceType = common.ReverseMap(interfaceTypeToString)
)

// DNSQueryType is modified from sing-box, by nekohasekai <contact-sagernet@sekai.icu>.
// https://github.com/SagerNet/sing-box/blob/fe92ab3e78a9bb7d448c155ef6906218e2ca5453/option/types.go
type DNSQueryType uint16

func (t DNSQueryType) String() string {
	name, ok := singdns.TypeToString[uint16(t)]
	if ok {
		return name
	}
	return strconv.FormatUint(uint64(t), 10)
}

func (t DNSQueryType) MarshalJSON() ([]byte, error) {
	name, ok := singdns.TypeToString[uint16(t)]
	if ok {
		return json.Marshal(name)
	}
	return json.Marshal(uint16(t))
}

func (t *DNSQueryType) UnmarshalJSON(data []byte) error {
	var number uint16
	if json.Unmarshal(data, &number) == nil {
		*t = DNSQueryType(number)
		return nil
	}
	var name string
	err := json.Unmarshal(data, &name)
	if err != nil {
		return fmt.Errorf("unmarshalJSON: unknown DNSQueryType: %s", data)
	}
	value, ok := singdns.StringToType[name]
	if !ok {
		return fmt.Errorf("unmarshalJSON: unknown DNSQueryType: %s", data)
	}
	*t = DNSQueryType(value)
	return nil
}

type RuleType string

const (
	RuleTypeDefault RuleType = "default"
	RuleTypeLogical RuleType = "logical"
)

func (r RuleType) String() string {
	switch r {
	case "":
		return string(RuleTypeDefault)
	case RuleTypeLogical, RuleTypeDefault:
		return string(r)
	default:
		return fmt.Sprintf("RuleType(%s)", string(r))
	}
}

func (r RuleType) MarshalJSON() ([]byte, error) {
	switch r {
	case RuleTypeDefault, "":
		// empty
		return make([]byte, 0), nil
	case RuleTypeLogical:
		return json.Marshal(RuleTypeLogical.String())
	default:
		return nil, fmt.Errorf("marshal: unknown RuleType: %s", string(r))
	}
}

func (r *RuleType) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	switch RuleType(s) {
	case RuleTypeLogical:
		*r = RuleTypeLogical
	case RuleTypeDefault, "":
		*r = RuleTypeDefault
	default:
		return fmt.Errorf("unmarshal: unknown RuleType: %s", s)
	}
	return nil
}

type LogicalRuleMode uint8

const (
	LogicalRuleModeAnd LogicalRuleMode = iota
	LogicalRuleModeOr
)

var (
	logicalRuleModeToString = map[LogicalRuleMode]string{
		LogicalRuleModeAnd: "and", LogicalRuleModeOr: "or",
	}
	stringToLogicalRuleMode = common.ReverseMap(logicalRuleModeToString)
)

func (o LogicalRuleMode) OK() bool {
	_, ok := logicalRuleModeToString[o]
	return ok
}

func (o LogicalRuleMode) String() string {
	oo, ok := logicalRuleModeToString[o]
	if ok {
		return oo
	}
	return fmt.Sprintf("LogicalRuleMode(%d)", o)
}

func ParseLogicalRuleMode(v string) (LogicalRuleMode, bool) {
	so, exist := stringToLogicalRuleMode[v]
	return so, exist
}

func (o LogicalRuleMode) MarshalJSON() ([]byte, error) {
	value, ok := logicalRuleModeToString[o]
	if !ok {
		return nil, fmt.Errorf("marshalJSON: invalid LogicalRuleMode: %d", o)
	}
	return json.Marshal(value)
}

func (o *LogicalRuleMode) UnmarshalJSON(data []byte) error {
	var value string
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}
	mode, ok := ParseLogicalRuleMode(value)
	if !ok {
		return fmt.Errorf("unmarshalJSON: invalid LogicalRuleMode: %s", value)
	}
	*o = mode
	return nil
}

type RuleSetVersion uint8

const (
	_rulesetVersionMin RuleSetVersion = iota
	RuleSetVersion1
	RuleSetVersion2
	RuleSetVersion3
	RuleSetVersion4
	RuleSetVersion5
	_rulesetVersionMax
	RuleSetVersionCurrent = RuleSetVersion5
)

func (r RuleSetVersion) OK() bool {
	return r < _rulesetVersionMax && r > _rulesetVersionMin
}

func (r RuleSetVersion) MarshalJSON() ([]byte, error) {
	if !r.OK() {
		return nil, fmt.Errorf("marshalJSON: invalid version: %d", r)
	}

	return []byte(strconv.FormatUint(uint64(r), 10)), nil
}

func (r *RuleSetVersion) UnmarshalJSON(data []byte) error {
	var u uint8
	err := json.Unmarshal(data, &u)
	if err != nil {
		return fmt.Errorf("unmarshalJSON: %w", err)
	}

	rr := RuleSetVersion(u)
	if rr.OK() {
		*r = rr
		return nil
	}
	return fmt.Errorf("unmarshalJSON: invalid RuleSetVersion: %d", u)
}
