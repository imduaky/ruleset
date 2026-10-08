package srs

import (
	"encoding/json"
	"fmt"

	"github.com/imduaky/ruleset/pkg/internal/singjson"
)

type Rule struct {
	Type           RuleType    `json:"type,omitempty"`
	DefaultOptions DefaultRule `json:"-"`
	LogicalOptions LogicalRule `json:"-"`
}

type defaultTypeRule struct {
	DefaultRule

	Type RuleType `json:"type,omitempty"` // should be empty
}

type logicalTypeRule struct {
	LogicalRule

	Type RuleType `json:"type,omitempty"` // should be "logical"
}

func (r *Rule) MarshalJSON() ([]byte, error) {
	switch r.Type {
	case RuleTypeDefault:
		return json.Marshal(defaultTypeRule{DefaultRule: r.DefaultOptions, Type: RuleTypeDefault})
	case RuleTypeLogical:
		return json.Marshal(logicalTypeRule{LogicalRule: r.LogicalOptions, Type: RuleTypeLogical})
	default:
		return nil, fmt.Errorf("unknown rule type: %s", r.Type)
	}
}

func (r *Rule) UnmarshalJSON(data []byte) error {
	type typedRule struct {
		Type RuleType `json:"type,omitempty"`
	}
	var typedRuleObject typedRule

	err := json.Unmarshal(data, &typedRuleObject)
	if err != nil {
		return err
	}

	newRule := new(Rule)
	newRule.Type = typedRuleObject.Type

	switch newRule.Type {
	case RuleTypeDefault:
		defaultRule := new(DefaultRule)
		err := json.Unmarshal(data, &defaultRule)
		if err != nil {
			return err
		}
		newRule.DefaultOptions = *defaultRule
	case RuleTypeLogical:
		logicalRule := new(LogicalRule)
		err := json.Unmarshal(data, &logicalRule)
		if err != nil {
			return err
		}
		newRule.LogicalOptions = *logicalRule
	default:
		return fmt.Errorf("unknown rule type: %s", newRule.Type)
	}

	*r = *newRule
	return nil
}

type DefaultRule struct {
	// In sing-box , QueryType will valid the value if legal, but I don't valid it here,
	// it will introduce some dns dependence to valid it, It is so heavy.
	QueryType singjson.Listable[string] `json:"query_type,omitempty"`

	Network              singjson.Listable[string]              `json:"network,omitempty" enum:"tcp,udp,icmp"`
	Domain               singjson.Listable[string]              `json:"domain,omitempty"`
	DomainSuffix         singjson.Listable[string]              `json:"domain_suffix,omitempty"`
	DomainKeyword        singjson.Listable[string]              `json:"domain_keyword,omitempty"`
	DomainRegex          singjson.Listable[string]              `json:"domain_regex,omitempty"`
	SourceIPCIDR         singjson.Listable[singjson.Prefixable] `json:"source_ip_cidr,omitempty"`
	IPCIDR               singjson.Listable[singjson.Prefixable] `json:"ip_cidr,omitempty"`
	SourcePort           singjson.Listable[uint16]              `json:"source_port,omitempty"`
	Port                 singjson.Listable[uint16]              `json:"port,omitempty"`
	SourcePortRange      singjson.Listable[string]              `json:"source_port_range,omitempty"`
	PortRange            singjson.Listable[string]              `json:"port_range,omitempty"`
	ProcessName          singjson.Listable[string]              `json:"process_name,omitempty"`
	ProcessPath          singjson.Listable[string]              `json:"process_path,omitempty"`
	ProcessPathRegex     singjson.Listable[string]              `json:"process_path_regex,omitempty"`
	PackageName          singjson.Listable[string]              `json:"package_name,omitempty"`
	PackageNameRegex     singjson.Listable[string]              `json:"package_name_regex,omitempty"`
	NetworkType          singjson.Listable[InterfaceType]       `json:"network_type,omitempty"`
	NetworkIsExpensive   bool                                   `json:"network_is_expensive,omitempty"`
	NetworkIsConstrained bool                                   `json:"network_is_constrained,omitempty"`
	WIFISSID             singjson.Listable[string]              `json:"wifi_ssid,omitempty"`
	WIFIBSSID            singjson.Listable[string]              `json:"wifi_bssid,omitempty"`

	// In sing-box , NetworkInterfaceAddress will use a linkedhashmap (ordered-map) to unmarshal, marshal.
	// but I think it is not important if the map is ordered, so I just use go map here.
	// https://github.com/SagerNet/sing-box/blob/b6c416b0482a2d2391470d70ce518abff3ba51f8/option/rule_set.go#L209
	NetworkInterfaceAddress map[InterfaceType]singjson.Prefixable `json:"network_interface_address,omitempty"`

	DefaultInterfaceAddress singjson.Listable[singjson.Prefixable] `json:"default_interface_address,omitempty"`

	Invert bool `json:"invert,omitempty"`
}

type LogicalRule struct {
	Mode LogicalRuleMode `json:"mode"`

	Rules  []Rule `json:"rules,omitempty"`
	Invert bool   `json:"invert,omitempty"`
}
