package common

// Charset is a set of bytes.
type Charset [256]bool

func NewCharset(chars string) *Charset {
	var set Charset
	for i := range len(chars) {
		set[chars[i]] = true
	}
	return &set
}

// All reports whether s is not empty and only consists of chars in the set.
func (set *Charset) All(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if !set[s[i]] {
			return false
		}
	}
	return true
}
