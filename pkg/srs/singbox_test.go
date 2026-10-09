package srs

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/stretchr/testify/require"
)

// Cross check with sing-box, the binary output must be byte-for-byte identical.

func singBoxWriteJSON(t *testing.T, content []byte) []byte {
	compat, err := json.UnmarshalExtendedContext[option.PlainRuleSetCompat](context.Background(), content)
	require.NoError(t, err)
	plain, err := compat.Upgrade()
	require.NoError(t, err)
	var buffer bytes.Buffer
	require.NoError(t, srs.Write(&buffer, plain, compat.Version))
	return buffer.Bytes()
}

// singBoxRewrite reads data and writes it again with sing-box.
func singBoxRewrite(t *testing.T, data []byte) []byte {
	compat, err := srs.Read(bytes.NewReader(data), true)
	require.NoError(t, err)
	plain, err := compat.Upgrade()
	require.NoError(t, err)
	var buffer bytes.Buffer
	require.NoError(t, srs.Write(&buffer, plain, compat.Version))
	return buffer.Bytes()
}

func rewrite(t *testing.T, data []byte) []byte {
	var ruleset Ruleset
	require.NoError(t, ruleset.UnmarshalSrsBinary(data))
	out, err := ruleset.MarshalSrsBinary()
	require.NoError(t, err)
	return out
}

func TestSingBoxBinary(t *testing.T) {
	ruleset := fullRuleset()
	data, err := ruleset.MarshalSrsBinary()
	require.NoError(t, err)
	require.Equal(t, data, singBoxRewrite(t, data))
}

func TestSingBoxJSON(t *testing.T) {
	ruleset := fullRuleset()
	// AdGuard rules are not part of the sing-box source format.
	ruleset.Rules[0].DefaultOptions.AdGuardDomain = nil
	content, err := ruleset.MarshalSrsJSON()
	require.NoError(t, err)

	data, err := ruleset.MarshalSrsBinary()
	require.NoError(t, err)
	singBoxData := singBoxWriteJSON(t, content)
	require.Equal(t, singBoxData, data)
	require.Equal(t, singBoxData, rewrite(t, singBoxData))
}

func TestSingBoxVersion(t *testing.T) {
	for _, content := range []string{
		`{"version":1,"rules":[{"domain":["a.example.com","example.org"],"domain_suffix":[".cn","example.net"],"ip_cidr":["1.1.1.0/24","::1"],"query_type":["A",65]}]}`,
		`{"version":2,"rules":[{"domain":"example.org","domain_suffix":["example.net"],"source_ip_cidr":"10.0.0.1"}]}`,
		`{"version":3,"rules":[{"network_type":"wifi","network_is_expensive":true},{"type":"logical","mode":"or","rules":[{"port":80}]}]}`,
		`{"version":4,"rules":[{"network_interface_address":{"wifi":"192.168.1.0/24"},"default_interface_address":"fe80::/10"}]}`,
		`{"version":4,"rules":[{"network_interface_address":{"other":"10.0.0.0/8","ethernet":["fe80::/10"],"wifi":"192.168.1.0/24"}}]}`,
	} {
		var ruleset Ruleset
		require.NoError(t, ruleset.UnmarshalSrsJson([]byte(content)))
		data, err := ruleset.MarshalSrsBinary()
		require.NoError(t, err)
		singBoxData := singBoxWriteJSON(t, []byte(content))
		require.Equal(t, singBoxData, data, content)
		require.Equal(t, singBoxData, rewrite(t, singBoxData), content)
	}
}

// TestSingBoxFiles checks real rule-set files, e.g.
// SRS_FILES="geosite-cn.srs geoip-cn.srs" go test -run TestSingBoxFiles ./pkg/srs
func TestSingBoxFiles(t *testing.T) {
	files := strings.Fields(os.Getenv("SRS_FILES"))
	if len(files) == 0 {
		t.Skip("SRS_FILES is not set")
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		require.Equal(t, singBoxRewrite(t, data), rewrite(t, data), file)
	}
}
