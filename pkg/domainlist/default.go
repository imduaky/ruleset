package domainlist

// Symbols of the domain list format, the same as v2fly/domain-list-community.
// https://github.com/v2fly/domain-list-community/blob/master/README.md
const (
	SymbolComment     = "#"
	SymbolType        = ":"
	SymbolAttribute   = "@"
	SymbolAffiliation = "&"
	SymbolBan         = "-" // `include:list @-attr` excludes the rules with attr

	PrefixInclude = "include"
)

type RuleType string

const (
	RuleTypeDomain  RuleType = "domain" // the type is omitted when writing
	RuleTypeFull    RuleType = "full"
	RuleTypeKeyword RuleType = "keyword"
	RuleTypeRegexp  RuleType = "regexp"
)

const (
	maxDomainLen = 253 // Maximum length of a domain name
	maxLabelLen  = 63  // Maximum length of a label of a domain name
)
