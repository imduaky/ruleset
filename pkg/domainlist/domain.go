package domainlist

import (
	"encoding"
	"fmt"
	"slices"
	"strings"

	"github.com/imduaky/ruleset/pkg/rulex"
)

var (
	_ encoding.TextAppender    = (*Domain)(nil)
	_ encoding.TextMarshaler   = (*Domain)(nil)
	_ encoding.TextUnmarshaler = (*Domain)(nil)
)

// Domain is a domain rule, e.g. `full:www.example.com @ads &other-list`.
type Domain struct {
	Type  RuleType
	Value string

	// Attrs are kept in the final lists, they are sorted and deduplicated when parsing.
	Attrs []string
	// Affiliations are the names of other lists which the rule is added to as well.
	Affiliations []string
}

// domainKey identifies a domain without affiliations, the same as the official.
type domainKey struct {
	typ   RuleType
	value string
	attrs string
}

func (d Domain) key() domainKey {
	// Attrs never contain spaces, and Join does not allocate for less than 2 attrs, which are the most.
	return domainKey{typ: d.Type, value: d.Value, attrs: strings.Join(d.Attrs, " ")}
}

func (d Domain) Equal(dd Domain) bool {
	return d.Type == dd.Type && d.Value == dd.Value &&
		slices.Equal(d.Attrs, dd.Attrs) &&
		slices.Equal(d.Affiliations, dd.Affiliations)
}

func (d Domain) String() string {
	return string(d.appendText(make([]byte, 0, d.textLen())))
}

// AppendText only checks the format, see Validate.
func (d Domain) AppendText(b []byte) ([]byte, error) {
	err := d.check()
	if err != nil {
		return b, err
	}
	return d.appendText(b), nil
}

func (d Domain) MarshalText() ([]byte, error) {
	data, err := d.AppendText(make([]byte, 0, d.textLen()))
	if err != nil {
		return nil, fmt.Errorf("marshalText: %w", err)
	}
	return data, nil
}

func (d *Domain) UnmarshalText(data []byte) error {
	typ, rule := cutType(strings.TrimSpace(string(data)))
	dd, err := parseDomain(RuleType(typ), rule)
	if err != nil {
		return fmt.Errorf("unmarshalText: %w", err)
	}
	*d = dd
	return nil
}

// parseDomain parses the rule after the type, the same as the official except that the value is not validated.
func parseDomain(typ RuleType, rule string) (Domain, error) {
	value, rest := cutField(rule)
	if typ != RuleTypeRegexp {
		value = strings.ToLower(value)
	}
	d := Domain{Type: typ, Value: value}
	for field := range strings.FieldsSeq(rest) {
		switch field[:1] {
		case SymbolAttribute:
			d.Attrs = append(d.Attrs, strings.ToLower(field[1:]))
		case SymbolAffiliation:
			d.Affiliations = append(d.Affiliations, strings.ToLower(field[1:]))
		default:
			return Domain{}, fmt.Errorf("unknown field: %q", field)
		}
	}
	slices.Sort(d.Attrs)
	d.Attrs = slices.Compact(d.Attrs)

	err := d.check()
	if err != nil {
		return Domain{}, err
	}
	return d, nil
}

// Validate reports whether the rule is valid, including its value, e.g. a domain name
// for RuleTypeDomain, while parsing and marshaling only check the format.
func (d Domain) Validate() error {
	err := d.check()
	if err != nil {
		return err
	}
	return validateValue(d.Type, d.Value)
}

// ToASCII converts the internationalized domain name of RuleTypeDomain and RuleTypeFull to punycode,
// e.g. "例子.com" to "xn--fsqu00a.com", see rulex.ToASCII. Values of other types are kept as is,
// since a part of a label of a keyword or regexp could not be converted alone.
func (d *Domain) ToASCII() error {
	value, err := d.asciiValue()
	if err != nil {
		return err
	}
	d.Value = value
	return nil
}

func (d Domain) asciiValue() (string, error) {
	switch d.Type {
	case RuleTypeDomain, RuleTypeFull:
		return rulex.ToASCII(d.Value)
	default:
		return d.Value, nil
	}
}

// check reports whether the rule could be written and read back, the value is not validated.
func (d Domain) check() error {
	if !isRuleType(d.Type) {
		return fmt.Errorf("unknown rule type: %q", d.Type)
	}
	if !isValue(d.Value) {
		return fmt.Errorf("invalid value: %q", d.Value)
	}
	err := validateAll(d.Attrs, isAttr, "attribute")
	if err != nil {
		return err
	}
	return validateAll(d.Affiliations, isListName, "affiliation")
}

func (d Domain) textLen() int {
	n := len(d.Value)
	if d.Type != RuleTypeDomain {
		n += len(d.Type) + len(SymbolType)
	}
	return n + fieldsLen(SymbolAttribute, d.Attrs) + fieldsLen(SymbolAffiliation, d.Affiliations)
}

func (d Domain) appendText(b []byte) []byte {
	if d.Type != RuleTypeDomain {
		b = append(b, d.Type...)
		b = append(b, SymbolType...)
	}
	b = append(b, d.Value...)
	b = appendFields(b, SymbolAttribute, d.Attrs)
	return appendFields(b, SymbolAffiliation, d.Affiliations)
}
