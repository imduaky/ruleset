package domainlist_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/imduaky/ruleset/pkg/domainlist"
)

func Example() {
	// A list file under the data directory of v2fly/domain-list-community, e.g. data/example.
	content := `# Example list
include:example-ads @-cn

Example.com
full:www.example.com @cn @ads
keyword:example
regexp:^odd[1-7]\.example\.org$
`
	list, err := domainlist.Read(strings.NewReader(content))
	if err != nil {
		panic(err)
	}
	for _, inc := range list.Inclusions {
		fmt.Printf("include %s, without %v\n", inc.Name, inc.BanAttrs)
	}
	for _, d := range list.Domains {
		fmt.Printf("%s %s %v\n", d.Type, d.Value, d.Attrs)
	}

	// Comments are dropped and values are normalized.
	err = domainlist.Write(os.Stdout, *list)
	if err != nil {
		panic(err)
	}
	// Output:
	// include example-ads, without [cn]
	// domain example.com []
	// full www.example.com [ads cn]
	// keyword example []
	// regexp ^odd[1-7]\.example\.org$ []
	// include:example-ads @-cn
	// example.com
	// full:www.example.com @ads @cn
	// keyword:example
	// regexp:^odd[1-7]\.example\.org$
}

func ExampleDomainList_MarshalText() {
	list := domainlist.DomainList{
		Inclusions: []domainlist.Inclusion{{Name: "example-cdn", MustAttrs: []string{"cn"}}},
		Domains: []domainlist.Domain{
			{Type: domainlist.RuleTypeDomain, Value: "example.com", Affiliations: []string{"example-all"}},
			{Type: domainlist.RuleTypeFull, Value: "ads.example.com", Attrs: []string{"ads"}},
		},
	}
	data, err := list.MarshalText()
	if err != nil {
		panic(err)
	}
	fmt.Print(string(data))

	// Values must be valid and lowercase.
	list.Domains[0].Value = "Example.com"
	_, err = list.MarshalText()
	fmt.Println(err)
	// Output:
	// include:example-cdn @cn
	// example.com &example-all
	// full:ads.example.com @ads
	// marshalText: domains[0]: invalid domain: "Example.com"
}

func ExampleDomain_UnmarshalText() {
	var d domainlist.Domain
	err := d.UnmarshalText([]byte("FULL:WWW.Example.com @cn @ads"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%q %q %q\n", d.Type, d.Value, d.Attrs)
	fmt.Println(d)

	err = d.UnmarshalText([]byte("example..com"))
	fmt.Println(err)
	// Output:
	// "full" "www.example.com" ["ads" "cn"]
	// full:www.example.com @ads @cn
	// unmarshalText: invalid domain: "example..com"
}
