package srs_test

import (
	"fmt"
	"net/netip"

	"github.com/imduaky/ruleset/pkg/srs"
	"github.com/imduaky/ruleset/pkg/srs/singjson"
)

func Example() {
	interfaceAddress := new(singjson.TypedMap[srs.InterfaceType, singjson.Listable[singjson.Prefixable]])
	interfaceAddress.Put(srs.InterfaceTypeWIFI, singjson.Listable[singjson.Prefixable]{
		singjson.Prefixable(netip.MustParsePrefix("192.168.1.0/24")),
	})
	ruleset := srs.Ruleset{
		Version: srs.RuleSetVersion4,
		Rules: []srs.Rule{{
			Type: srs.RuleTypeDefault,
			DefaultOptions: srs.DefaultRule{
				QueryType:               singjson.Listable[srs.DNSQueryType]{1, 28},
				DomainSuffix:            singjson.Listable[string]{"example.com"},
				NetworkInterfaceAddress: interfaceAddress,
			},
		}},
	}

	content, err := ruleset.MarshalSrsJSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(string(content))

	data, err := ruleset.MarshalSrsBinary()
	if err != nil {
		panic(err)
	}
	var decoded srs.Ruleset
	err = decoded.UnmarshalSrsBinary(data)
	if err != nil {
		panic(err)
	}
	for _, entry := range decoded.Rules[0].DefaultOptions.NetworkInterfaceAddress.Entries() {
		for _, prefix := range entry.Value {
			fmt.Println(entry.Key, netip.Prefix(prefix))
		}
	}
	// Output:
	// {"version":4,"rules":[{"query_type":["A","AAAA"],"domain_suffix":"example.com","network_interface_address":{"wifi":"192.168.1.0/24"}}]}
	// wifi 192.168.1.0/24
}
