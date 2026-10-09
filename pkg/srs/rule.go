package srs

import (
	"encoding/json/v2"
	"fmt"

	"github.com/imduaky/ruleset/pkg/srs/singjson"
)

type Rule struct {
	Type           RuleType    `json:"type,omitempty"`
	DefaultOptions DefaultRule `json:"-"`
	LogicalOptions LogicalRule `json:"-"`
}

type defaultTypeRule struct {
	DefaultRule

	Type RuleType `json:"type,omitzero"` // should be empty
}

type logicalTypeRule struct {
	LogicalRule

	Type RuleType `json:"type,omitempty"` // should be "logical"
}

func (r *Rule) MarshalJSON() ([]byte, error) {
	switch r.Type {
	case RuleTypeDefault, "":
		return json.Marshal(defaultTypeRule{DefaultRule: r.DefaultOptions})
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
	if newRule.Type == "" {
		newRule.Type = RuleTypeDefault
	}

	switch newRule.Type {
	case RuleTypeDefault:
		var defaultRule defaultTypeRule
		err := singjson.UnmarshalStrict(data, &defaultRule)
		if err != nil {
			return err
		}
		newRule.DefaultOptions = defaultRule.DefaultRule
	case RuleTypeLogical:
		var logicalRule logicalTypeRule
		err := singjson.UnmarshalStrict(data, &logicalRule)
		if err != nil {
			return err
		}
		newRule.LogicalOptions = logicalRule.LogicalRule
	default:
		return fmt.Errorf("unknown rule type: %s", newRule.Type)
	}

	*r = *newRule
	return nil
}

type DefaultRule struct {
	QueryType singjson.Listable[DNSQueryType] `json:"query_type,omitempty"`

	Network              singjson.Listable[string]              `json:"network,omitempty" enum:"tcp,udp,icmp"`
	Domain               singjson.Listable[string]              `json:"domain,omitempty"`
	DomainSuffix         singjson.Listable[string]              `json:"domain_suffix,omitempty"`
	DomainKeyword        singjson.Listable[string]              `json:"domain_keyword,omitempty"`
	DomainRegex          singjson.Listable[string]              `json:"domain_regex,omitempty"`
	AdGuardDomain        singjson.Listable[string]              `json:"adguard_domain,omitempty"`
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
	NetworkIsExpensive   bool                                   `json:"network_is_expensive,omitzero"`
	NetworkIsConstrained bool                                   `json:"network_is_constrained,omitzero"`
	WIFISSID             singjson.Listable[string]              `json:"wifi_ssid,omitempty"`
	WIFIBSSID            singjson.Listable[string]              `json:"wifi_bssid,omitempty"`

	// Same as sing-box, the order of entries is kept, so the input and output are the same.
	// https://github.com/SagerNet/sing-box/blob/b6c416b0482a2d2391470d70ce518abff3ba51f8/option/rule_set.go#L209
	NetworkInterfaceAddress *singjson.TypedMap[InterfaceType, singjson.Listable[singjson.Prefixable]] `json:"network_interface_address,omitempty"`

	DefaultInterfaceAddress singjson.Listable[singjson.Prefixable] `json:"default_interface_address,omitempty"`

	Invert bool `json:"invert,omitzero"`
}

type LogicalRule struct {
	Mode LogicalRuleMode `json:"mode"`

	Rules  []Rule `json:"rules,omitempty"`
	Invert bool   `json:"invert,omitzero"`
}
