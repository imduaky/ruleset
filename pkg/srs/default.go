package srs

import (
	"fmt"
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

const (
	RuleTypeDefault = "default"
	RuleTypeLogical = "logical"
)
