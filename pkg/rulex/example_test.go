package rulex_test

import (
	"errors"
	"fmt"

	"github.com/imduaky/ruleset/pkg/rulex"
)

func Example() {
	fmt.Println(rulex.IsDomain("www.example.com"), rulex.IsDomain("example..com"))
	fmt.Println(rulex.ValidateKeyword("goo_gle"))

	err := rulex.ValidateRegexp(`^(www\.)?example\.com`)
	fmt.Println(err)
	err = rulex.ValidateRegexp(`^(www\.example\.com`)
	fmt.Println(errors.Is(err, rulex.ErrInvalidRegexp))
	// Output:
	// true false
	// invalid keyword: "goo_gle"
	// <nil>
	// true
}

func ExampleToASCII() {
	// Internationalized domain names are valid without being converted to punycode.
	fmt.Println(rulex.IsDomain("例子.com"), rulex.IsDomain("münchen.de"))

	for _, domain := range []string{"例子.com", "MÜNCHEN.de", "例子。com", "example.com"} {
		ascii, err := rulex.ToASCII(domain)
		if err != nil {
			panic(err)
		}
		fmt.Println(ascii, rulex.IsDomain(ascii))
	}
	// Output:
	// true true
	// xn--fsqu00a.com true
	// xn--mnchen-3ya.de true
	// xn--fsqu00a.com true
	// example.com true
}
