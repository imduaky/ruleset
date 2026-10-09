// Modified from sing-box, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing-box/blob/fe92ab3e78a9bb7d448c155ef6906218e2ca5453/cmd/sing-box/cmd_rule_set_compile.go

package srs

import "github.com/imduaky/ruleset/pkg/internal/common"

func DowngradeRuleSetVersion(version RuleSetVersion, rules []Rule) RuleSetVersion {
	if version >= RuleSetVersion5 && !RulesHasFunc(rules, func(rr DefaultRule) bool {
		return len(rr.PackageNameRegex) > 0
	}) {
		version = RuleSetVersion4
	}

	if version >= RuleSetVersion4 && !RulesHasFunc(rules, func(rr DefaultRule) bool {
		return rr.NetworkInterfaceAddress != nil && rr.NetworkInterfaceAddress.Size() > 0 || len(rr.DefaultInterfaceAddress) > 0
	}) {
		version = RuleSetVersion3
	}

	if version >= RuleSetVersion3 && !RulesHasFunc(rules, func(rr DefaultRule) bool {
		return len(rr.NetworkType) > 0 || rr.NetworkIsExpensive || rr.NetworkIsConstrained
	}) {
		version = RuleSetVersion2
	}

	return version
}

func RulesHasFunc(r []Rule, f func(rr DefaultRule) bool) bool {
	return common.Or(r, func(rule Rule) bool {
		switch rule.Type {
		case RuleTypeDefault, "":
			return f(rule.DefaultOptions)
		case RuleTypeLogical:
			return RulesHasFunc(rule.LogicalOptions.Rules, f)
		default:
			panic("invalid RuleType")
		}
	})
}
