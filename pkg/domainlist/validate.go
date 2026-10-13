package domainlist

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/imduaky/ruleset/pkg/internal/common"
	"github.com/imduaky/ruleset/pkg/rulex"
)

const lowerDigits = "abcdefghijklmnopqrstuvwxyz0123456789"

// Names are checked after being lowercased, while the official checks list names after being uppercased.
var (
	attrChars     = common.NewCharset(lowerDigits + "!")
	listNameChars = common.NewCharset(lowerDigits + "!-")
)

func isAttr(s string) bool { return attrChars.All(s) }

func isListName(s string) bool { return listNameChars.All(s) }

func isRuleType(typ RuleType) bool {
	switch typ {
	case RuleTypeDomain, RuleTypeFull, RuleTypeKeyword, RuleTypeRegexp:
		return true
	}
	return false
}

func isValue(s string) bool {
	return s != "" && !strings.ContainsFunc(s, unicode.IsSpace) && !strings.Contains(s, SymbolComment)
}

func validateValue(typ RuleType, value string) error {
	switch typ {
	case RuleTypeDomain, RuleTypeFull:
		return rulex.ValidateDomain(value)
	case RuleTypeKeyword:
		return rulex.ValidateKeyword(value)
	case RuleTypeRegexp:
		return rulex.ValidateRegexp(value)
	default:
		return fmt.Errorf("unknown rule type: %q", typ)
	}
}

func validateAll(values []string, valid func(string) bool, name string) error {
	for _, value := range values {
		if !valid(value) {
			return fmt.Errorf("invalid %s: %q", name, value)
		}
	}
	return nil
}
