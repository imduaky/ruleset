package domainlist_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing/fstest"

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

	// Values must not contain spaces, otherwise they could not be read back.
	list.Domains[0].Value = "example .com"
	_, err = list.MarshalText()
	fmt.Println(err)
	// Output:
	// include:example-cdn @cn
	// example.com &example-all
	// full:ads.example.com @ads
	// marshalText: domains[0]: invalid value: "example .com"
}

func ExampleDomain_UnmarshalText() {
	var d domainlist.Domain
	err := d.UnmarshalText([]byte("FULL:WWW.Example.com @cn @ads"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%q %q %q\n", d.Type, d.Value, d.Attrs)
	fmt.Println(d)

	// Only the format is checked, the value is validated by Validate.
	err = d.UnmarshalText([]byte("example..com"))
	fmt.Println(err)
	fmt.Println(d.Validate())
	// Output:
	// "full" "www.example.com" ["ads" "cn"]
	// full:www.example.com @ads @cn
	// <nil>
	// invalid domain: "example..com"
}

func ExampleDomainList_Validate() {
	list, err := domainlist.Read(strings.NewReader(`
example.com
full:www..example.com
keyword:goo_gle
`))
	if err != nil {
		panic(err)
	}
	for _, d := range list.Domains {
		fmt.Printf("%s: %v\n", d, d.Validate())
	}
	fmt.Println(list.Validate())
	// Output:
	// example.com: <nil>
	// full:www..example.com: invalid domain: "www..example.com"
	// keyword:goo_gle: invalid keyword: "goo_gle"
	// domains[1]: invalid domain: "www..example.com"
}

func ExampleMerger() {
	read := func(name, content string) domainlist.NamedDomainList {
		list, err := domainlist.Read(strings.NewReader(content))
		if err != nil {
			panic(err)
		}
		return domainlist.NamedDomainList{Name: name, DomainList: *list}
	}
	example := read("example", "include:example-ads @-cn\ninclude:example-cdn\nexample.com")
	ads := read("example-ads", "ads.example.com @ads\nads.example.cn @ads @cn")
	cdn := read("example-cdn", "include:example-ads @cn\ncdn.example.com &example-all")

	m := domainlist.NewMerger(domainlist.NewDomainListSet(example, ads, cdn))
	full, err := m.Merge(example.Name)
	if err != nil {
		panic(err)
	}
	err = domainlist.Write(os.Stdout, full)
	if err != nil {
		panic(err)
	}

	// The domains with the attributes, the same as `include:example @ads @-cn`.
	filtered, err := m.MergeAttrs(example.Name, "ads", "-cn")
	if err != nil {
		panic(err)
	}
	fmt.Println(filtered.Domains)

	// MergeOnce merges the inclusions once, whose inclusions are kept.
	once, err := m.MergeOnce(example.Name)
	if err != nil {
		panic(err)
	}
	fmt.Println(once.Inclusions)

	_, err = m.Merge("missing")
	fmt.Println(err, errors.Is(err, domainlist.ErrListNotFound))
	// Output:
	// example.com
	// ads.example.com @ads
	// cdn.example.com
	// ads.example.cn @ads @cn
	// [ads.example.com @ads]
	// [include:example-ads @cn]
	// list not found: missing true
}

func ExampleMerger_MergeAll() {
	a := domainlist.NamedDomainList{Name: "a", DomainList: domainlist.DomainList{
		Inclusions: []domainlist.Inclusion{{Name: "b"}},
		Domains:    []domainlist.Domain{{Type: domainlist.RuleTypeDomain, Value: "a.com"}},
	}}
	b := domainlist.NamedDomainList{Name: "b", DomainList: domainlist.DomainList{
		Domains: []domainlist.Domain{{Type: domainlist.RuleTypeFull, Value: "b.com"}},
	}}
	set := domainlist.NewDomainListSet(a, b)
	all, err := domainlist.NewMerger(set).MergeAll()
	if err != nil {
		panic(err)
	}
	for list, err := range all.All() {
		if err != nil {
			panic(err)
		}
		fmt.Println(list.Name, list.Domains)
	}
	// The looked up lists are not modified.
	list, err := set.Lookup("a")
	if err != nil {
		panic(err)
	}
	fmt.Println(list.Inclusions)

	// It stops at a circular inclusion.
	b.DomainList = domainlist.DomainList{Inclusions: []domainlist.Inclusion{{Name: "a"}}}
	_, err = domainlist.NewMerger(domainlist.NewDomainListSet(a, b)).MergeAll()
	fmt.Println(err)
	// Output:
	// a [a.com full:b.com]
	// b [full:b.com]
	// [include:b]
	// circular inclusion: [a -> b -> a]
}

func ExampleFileLookuper() {
	// The data directory of v2fly/domain-list-community, e.g. os.DirFS("domain-list-community/data").
	data := fstest.MapFS{
		"example":     {Data: []byte("include:example-ads\nexample.com")},
		"example-ads": {Data: []byte("ads.example.com @ads")},
		"other":       {Data: []byte("other.com")},
	}
	// The files are read when the lists are looked up, "other" is not read.
	list, err := domainlist.NewMerger(&domainlist.FileLookuper{FS: data}).Merge("example")
	if err != nil {
		panic(err)
	}
	fmt.Println(list.Domains)
	// Output:
	// [example.com ads.example.com @ads]
}

func ExampleDomainList_ToASCII() {
	list, err := domainlist.Read(strings.NewReader(`
例子.com
full:MÜNCHEN.de @de
`))
	if err != nil {
		panic(err)
	}
	// Internationalized domain names are valid without being converted to punycode.
	fmt.Println(list.Validate())

	err = list.ToASCII()
	if err != nil {
		panic(err)
	}
	err = domainlist.Write(os.Stdout, *list)
	if err != nil {
		panic(err)
	}
	// Output:
	// <nil>
	// xn--fsqu00a.com
	// full:xn--mnchen-3ya.de @de
}
