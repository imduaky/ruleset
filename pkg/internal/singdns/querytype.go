package singdns

import "github.com/imduaky/ruleset/pkg/internal/common"

// Generated from the latest miekg/dns, use -version to pin a version.
//go:generate go run gen.go

// StringToType is the reverse of TypeToString.
var StringToType = common.ReverseMap(TypeToString)
