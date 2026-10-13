package domainlist

import "slices"

const (
	// minMapAttrs is the minimum number of must and ban attributes looked up in maps, fewer ones are
	// faster to look up in slices.
	minMapAttrs = 9
	// maxMustBits is the maximum number of must attributes matched by the bits of a uint64.
	maxMustBits = 64
)

// attrFilter matches the attributes with all of the must and none of the ban attributes, the same as
// `include:list @must @-ban`. Many must and ban attributes are looked up in maps, so that matching
// takes linear time of the attributes, however many the must and ban attributes are.
//
// Note: There is a drawback here: performance drops sharply when the number of attributes exceeds 64,
// because a single bitmap can represent a maximum of 64 bits.
type attrFilter struct {
	must, ban []string

	// bitmap
	mustBits map[string]uint64 // a bit of each must attribute, if looked up in maps
	allBits  uint64            // the bits of all must attributes
	banSet   map[string]struct{}
}

// newAttrFilter returns a value instead of a pointer, which would escape to the heap since it is not
// inlined, so that the filter of an inclusion is on the stack of the caller.
func newAttrFilter(must, ban []string) attrFilter {
	f := attrFilter{must: must, ban: ban}
	if len(must)+len(ban) < minMapAttrs {
		return f
	}
	bits := make(map[string]uint64, len(must))
	for _, attr := range must {
		if _, ok := bits[attr]; !ok {
			// reached bitmap max(64)
			if len(bits) == maxMustBits {
				return f
			}
			bits[attr] = 1 << len(bits)
		}
	}
	f.mustBits, f.allBits = bits, 1<<len(bits)-1
	if len(bits) == maxMustBits {
		f.allBits = ^uint64(0)
	}
	f.banSet = make(map[string]struct{}, len(ban))
	for _, attr := range ban {
		f.banSet[attr] = struct{}{}
	}
	return f
}

func (f *attrFilter) match(attrs []string) bool {
	if f == nil {
		return true
	}
	if f.mustBits == nil {
		return matchAttrsSerial(attrs, f.must, f.ban)
	}

	var got uint64
	for _, attr := range attrs {
		if _, ok := f.banSet[attr]; ok {
			return false
		}
		got |= f.mustBits[attr]
	}
	return got == f.allBits
}

func matchAttrsSerial(attrs, must, ban []string) bool {
	for _, attr := range must {
		if !slices.Contains(attrs, attr) {
			return false
		}
	}
	for _, attr := range ban {
		if slices.Contains(attrs, attr) {
			return false
		}
	}
	return true
}

// none reports whether the filter has no attributes, which matches all.
func (f *attrFilter) none() bool {
	return f == nil || len(f.must) == 0 && len(f.ban) == 0
}
