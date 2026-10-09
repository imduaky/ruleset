package srs

import (
	"net/netip"
	"testing"

	"github.com/imduaky/ruleset/pkg/srs/singjson"

	"github.com/stretchr/testify/require"
)

func prefixes(values ...string) singjson.Listable[singjson.Prefixable] {
	result := make(singjson.Listable[singjson.Prefixable], len(values))
	for i, value := range values {
		result[i] = singjson.Prefixable(netip.MustParsePrefix(value))
	}
	return result
}

// interfaceAddress builds the map in order from key, value pairs.
func interfaceAddress(pairs ...any) *singjson.TypedMap[InterfaceType, singjson.Listable[singjson.Prefixable]] {
	m := new(singjson.TypedMap[InterfaceType, singjson.Listable[singjson.Prefixable]])
	for i := 0; i < len(pairs); i += 2 {
		m.Put(pairs[i].(InterfaceType), pairs[i+1].(singjson.Listable[singjson.Prefixable]))
	}
	return m
}

func fullRuleset() Ruleset {
	return Ruleset{
		Version: RuleSetVersionCurrent,
		Rules: []Rule{
			{Type: RuleTypeDefault, DefaultOptions: DefaultRule{
				QueryType:            []DNSQueryType{1, 28, 65, 4096},
				Network:              []string{"tcp", "udp"},
				Domain:               []string{"a.example.com", "example.org"},
				DomainSuffix:         []string{".cn", "example.net"},
				DomainKeyword:        []string{"google"},
				DomainRegex:          []string{`^.*\.example$`},
				AdGuardDomain:        []string{"ads", "|example.org^", "||example.com^"},
				SourceIPCIDR:         prefixes("10.0.0.0/8", "fd00::/8"),
				IPCIDR:               prefixes("1.1.1.0/24", "2001:db8::/32"),
				SourcePort:           []uint16{1234},
				SourcePortRange:      []string{"1000:2000"},
				Port:                 []uint16{53, 443},
				PortRange:            []string{":3000"},
				ProcessName:          []string{"curl"},
				ProcessPath:          []string{"/usr/bin/curl"},
				ProcessPathRegex:     []string{"^/usr/.*"},
				PackageName:          []string{"com.example"},
				PackageNameRegex:     []string{`^com\.example\..*`},
				NetworkType:          []InterfaceType{InterfaceTypeWIFI, InterfaceTypeCellular},
				NetworkIsExpensive:   true,
				NetworkIsConstrained: true,
				WIFISSID:             []string{"home"},
				WIFIBSSID:            []string{"00:00:00:00:00:00"},
				NetworkInterfaceAddress: interfaceAddress(
					InterfaceTypeEthernet, prefixes("fe80::/10"),
					InterfaceTypeWIFI, prefixes("192.168.1.0/24", "10.0.0.0/8"),
				),
				DefaultInterfaceAddress: prefixes("172.16.0.0/12"),
				Invert:                  true,
			}},
			{Type: RuleTypeLogical, LogicalOptions: LogicalRule{
				Mode: LogicalRuleModeAnd,
				Rules: []Rule{
					{Type: RuleTypeDefault, DefaultOptions: DefaultRule{Port: []uint16{80}}},
					{Type: RuleTypeLogical, LogicalOptions: LogicalRule{
						Mode:   LogicalRuleModeOr,
						Rules:  []Rule{{Type: RuleTypeDefault, DefaultOptions: DefaultRule{Network: []string{"udp"}}}},
						Invert: true,
					}},
				},
			}},
		},
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	ruleset := fullRuleset()
	data, err := ruleset.MarshalSrsBinary()
	require.NoError(t, err)

	var decoded Ruleset
	require.NoError(t, decoded.UnmarshalSrsBinary(data))
	require.Equal(t, ruleset, decoded)

	again, err := decoded.MarshalSrsBinary()
	require.NoError(t, err)
	require.Equal(t, data, again)
}

func TestBinaryAdGuardTrimDot(t *testing.T) {
	// Same as sing, the trailing dot of a rule without `^` is trimmed.
	ruleset := Ruleset{Version: RuleSetVersion2, Rules: []Rule{{Type: RuleTypeDefault, DefaultOptions: DefaultRule{
		AdGuardDomain: []string{"ads."},
	}}}}
	data, err := ruleset.MarshalSrsBinary()
	require.NoError(t, err)

	var decoded Ruleset
	require.NoError(t, decoded.UnmarshalSrsBinary(data))
	require.Equal(t, singjson.Listable[string]{"ads"}, decoded.Rules[0].DefaultOptions.AdGuardDomain)
}

func TestBinaryLegacyDomainSuffix(t *testing.T) {
	ruleset := Ruleset{Version: RuleSetVersion1, Rules: []Rule{{Type: RuleTypeDefault, DefaultOptions: DefaultRule{
		Domain:       []string{"example.org"},
		DomainSuffix: []string{"example.com", ".cn"},
	}}}}
	data, err := ruleset.MarshalSrsBinary()
	require.NoError(t, err)

	var decoded Ruleset
	require.NoError(t, decoded.UnmarshalSrsBinary(data))
	rule := decoded.Rules[0].DefaultOptions
	require.Equal(t, singjson.Listable[string]{"example.org"}, rule.Domain)
	require.Equal(t, singjson.Listable[string]{".cn", "example.com"}, rule.DomainSuffix)
}

func TestBinaryItemVersion(t *testing.T) {
	for _, rule := range []DefaultRule{
		{AdGuardDomain: []string{"||example.com^"}},
		{NetworkType: []InterfaceType{InterfaceTypeWIFI}},
		{NetworkIsExpensive: true},
		{DefaultInterfaceAddress: prefixes("10.0.0.0/8")},
		{PackageNameRegex: []string{".*"}},
	} {
		ruleset := Ruleset{Version: RuleSetVersion1, Rules: []Rule{{Type: RuleTypeDefault, DefaultOptions: rule}}}
		_, err := ruleset.MarshalSrsBinary()
		require.ErrorContains(t, err, "requires SRS version", "%+v", rule)
	}
}

func TestBinaryRejectsEmptyDomain(t *testing.T) {
	for _, rule := range []DefaultRule{{Domain: []string{""}}, {DomainSuffix: []string{""}}, {AdGuardDomain: []string{""}}} {
		ruleset := Ruleset{Version: RuleSetVersion2, Rules: []Rule{{Type: RuleTypeDefault, DefaultOptions: rule}}}
		_, err := ruleset.MarshalSrsBinary()
		require.Error(t, err, "%+v", rule)
	}
}

func TestBinaryRejectsMalformed(t *testing.T) {
	data, err := fullRuleset().MarshalSrsBinary()
	require.NoError(t, err)
	for name, value := range map[string][]byte{
		"magic":     append([]byte("XRS"), data[3:]...),
		"version":   append([]byte{'S', 'R', 'S', 0}, data[4:]...),
		"truncated": data[:len(data)/2],
		"trailing":  append(data[:len(data):len(data)], 0),
	} {
		var decoded Ruleset
		require.Error(t, decoded.UnmarshalSrsBinary(value), name)
	}
}

func TestBinaryLogicalDepth(t *testing.T) {
	rule := Rule{Type: RuleTypeDefault, DefaultOptions: DefaultRule{Port: []uint16{80}}}
	for range maxLogicalRuleDepth + 1 {
		rule = Rule{Type: RuleTypeLogical, LogicalOptions: LogicalRule{Mode: LogicalRuleModeAnd, Rules: []Rule{rule}}}
	}
	ruleset := Ruleset{Version: RuleSetVersion1, Rules: []Rule{rule}}
	_, err := ruleset.MarshalSrsBinary()
	require.ErrorContains(t, err, "too deep")
}

func TestJSONQueryType(t *testing.T) {
	var ruleset Ruleset
	require.NoError(t, ruleset.UnmarshalSrsJson([]byte(`{"version":3,"rules":[{"query_type":["A",28,"HTTPS",4096]}]}`)))
	require.Equal(t, singjson.Listable[DNSQueryType]{1, 28, 65, 4096}, ruleset.Rules[0].DefaultOptions.QueryType)

	data, err := ruleset.MarshalSrsJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"version":3,"rules":[{"query_type":["A","AAAA","HTTPS",4096]}]}`, string(data))

	require.Error(t, ruleset.UnmarshalSrsJson([]byte(`{"version":3,"rules":[{"query_type":"NOPE"}]}`)))
}

func TestJSONBinaryRoundTrip(t *testing.T) {
	ruleset := fullRuleset()
	data, err := ruleset.MarshalSrsJSON()
	require.NoError(t, err)

	var decoded Ruleset
	require.NoError(t, decoded.UnmarshalSrsJson(data))
	require.Equal(t, ruleset, decoded)
}
