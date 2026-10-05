package srs

import "C"
import "C"
import "C"
import "C"
import "C"
import "C"
import (
	"encoding/json"
	"errors"

	"github.com/imduaky/ruleset/pkg/internal/singjson"
)

type Rule struct {
	Type           string       `json:"type,omitempty"`
	DefaultOptions *DefaultRule `json:"-"`
	LogicalOptions *LogicalRule `json:"-"`
}

type noMethodRule Rule

func (r Rule) MarshalJSON() ([]byte, error) {
	var v any
	switch r.Type {
	case RuleTypeDefault, "":
		v = r.DefaultOptions
	case RuleTypeLogical:
		v = r.LogicalOptions
	default:
		return nil, errors.New("unknown rule type: " + r.Type)
	}

	return json.Marshal(v)
}

func (r *Rule) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*noMethodRule)(r))
	if err != nil {
		return err
	}
	var v any
	switch r.Type {
	case "", RuleTypeDefault:
		r.Type = C.RuleTypeDefault
		v = new(DefaultRule)
	case RuleTypeLogical:
		v = new(LogicalRule)
	default:
		return errors.New("unknown rule type: " + r.Type)
	}
	err = json.Unmarshal(bytes, v)
	if err != nil {
		return err
	}
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
	// but I think it is not important if the map is ordered, so we just use go map here.
	// https://github.com/SagerNet/sing-box/blob/b6c416b0482a2d2391470d70ce518abff3ba51f8/option/rule_set.go#L209
	NetworkInterfaceAddress map[InterfaceType]singjson.Prefixable `json:"network_interface_address,omitempty"`

	DefaultInterfaceAddress singjson.Listable[singjson.Prefixable] `json:"default_interface_address,omitempty"`

	Invert bool `json:"invert,omitempty"`
}

type LogicalRule struct {
	Mode   string `json:"mode"`
	Rules  []Rule `json:"rules,omitempty"`
	Invert bool   `json:"invert,omitempty"`
}
