package srs

import (
	"bytes"
	"compress/zlib"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

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

// compressRules writes a rule-set with extra data after the rules in the compressed stream.
func compressRules(t *testing.T, ruleset Ruleset, extra string) []byte {
	var content bytes.Buffer
	require.NoError(t, writeRules(&content, ruleset.Rules, ruleset.Version, 0))
	content.WriteString(extra)

	var buffer bytes.Buffer
	buffer.Write(append(magicBytes[:], byte(ruleset.Version)))
	w := zlib.NewWriter(&buffer)
	_, err := w.Write(content.Bytes())
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buffer.Bytes()
}

func TestBinaryRejectsMalformed(t *testing.T) {
	data, err := fullRuleset().MarshalSrsBinary()
	require.NoError(t, err)
	for name, value := range map[string]struct {
		data    []byte
		message string
	}{
		"magic":     {append([]byte("XRS"), data[3:]...), "not a srs"},
		"version":   {append([]byte{'S', 'R', 'S', 0}, data[4:]...), "invalid version"},
		"empty":     {nil, io.ErrUnexpectedEOF.Error()},
		"header":    {[]byte("SRS"), io.ErrUnexpectedEOF.Error()},
		"truncated": {data[:len(data)/2], io.ErrUnexpectedEOF.Error()},
		"extra":     {compressRules(t, fullRuleset(), "extra"), "unexpected data after rules"},
	} {
		var decoded Ruleset
		require.ErrorContains(t, decoded.UnmarshalSrsBinary(value.data), value.message, name)
	}
}

// windowRuleset returns a rule-set whose rules are exactly the 32KB window of flate,
// the last rules are decompressed before the end of the zlib stream is read.
func windowRuleset(t *testing.T) Ruleset {
	keywords := make(singjson.Listable[string], 1000)
	for i := range keywords {
		keywords[i] = "keyword-" + strconv.Itoa(i)
	}
	ruleset := Ruleset{Version: RuleSetVersion1, Rules: []Rule{{Type: RuleTypeDefault, DefaultOptions: DefaultRule{DomainKeyword: keywords}}}}
	for {
		var content bytes.Buffer
		require.NoError(t, writeRules(&content, ruleset.Rules, ruleset.Version, 0))
		diff := 1<<15 - content.Len()
		if diff == 0 {
			return ruleset
		}
		keywords[0] = strings.Repeat("a", len(keywords[0])+diff)
	}
}

func TestBinaryChecksum(t *testing.T) {
	// Whether zlib reaches the end of its stream with the last rule or not.
	for name, ruleset := range map[string]Ruleset{"full": fullRuleset(), "window": windowRuleset(t)} {
		data, err := ruleset.MarshalSrsBinary()
		require.NoError(t, err)
		var decoded Ruleset
		require.NoError(t, decoded.UnmarshalSrsBinary(data), name)
		require.Equal(t, ruleset, decoded, name)

		corrupt := bytes.Clone(data)
		corrupt[len(corrupt)-1] ^= 0xFF
		require.ErrorIs(t, decoded.UnmarshalSrsBinary(corrupt), zlib.ErrChecksum, name)
		require.ErrorIs(t, decoded.UnmarshalSrsBinary(data[:len(data)-4]), io.ErrUnexpectedEOF, name)
	}
}

func TestBinaryIgnoresTrailingData(t *testing.T) {
	// Same as sing-box, the data after the zlib stream is not checked.
	data, err := fullRuleset().MarshalSrsBinary()
	require.NoError(t, err)
	var decoded Ruleset
	require.NoError(t, decoded.UnmarshalSrsBinary(append(data[:len(data):len(data)], "trailing"...)))
	require.Equal(t, fullRuleset(), decoded)
}

func TestBinaryReadHeaderError(t *testing.T) {
	data, err := fullRuleset().MarshalSrsBinary()
	require.NoError(t, err)
	// ErrTimeout comes after the header is half read, it must be returned rather than ignored.
	_, err = Read(iotest.TimeoutReader(iotest.OneByteReader(bytes.NewReader(data))))
	require.ErrorIs(t, err, iotest.ErrTimeout)
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
