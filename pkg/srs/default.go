package srs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/imduaky/ruleset/pkg/internal/common"
)

var magicBytes = [3]byte{0x53, 0x52, 0x53} // SRS

func MagicBytes() []byte {
	return magicBytes[:]
}

const (
	RuleItemQueryType uint8 = iota
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

var (
	interfaceTypeToString = map[InterfaceType]string{
		InterfaceTypeWIFI:     "wifi",
		InterfaceTypeCellular: "cellular",
		InterfaceTypeEthernet: "ethernet",
		InterfaceTypeOther:    "other",
	}

	stringToInterfaceType = common.ReverseMap(interfaceTypeToString)
)

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
	case RuleTypeLogical, RuleTypeDefault, "":
		return []byte(r.String()), nil
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
	LogicalRuleModeOr LogicalRuleMode = iota
	LogicalRuleModeAnd
)

var (
	logicalRuleModeToString = map[LogicalRuleMode]string{
		LogicalRuleModeAnd: "and", LogicalRuleModeOr: "or",
	}
	stringToLogicalRuleMode = common.ReverseMap(logicalRuleModeToString)
)

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

type RuleSetVersion uint8

const (
	RuleSetVersion1 RuleSetVersion = 1 + iota
	RuleSetVersion2
	RuleSetVersion3
	RuleSetVersion4
	RuleSetVersion5
	_rulesetVersionMax
	RuleSetVersionCurrent = RuleSetVersion5
)

func (r RuleSetVersion) OK() bool {
	return r < _rulesetVersionMax
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
