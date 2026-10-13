package domainlist

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrListNotFound      = errors.New("list not found")
	ErrCircularInclusion = errors.New("circular inclusion")
)

const (
	// maxErrorPath is the maximum number of the lists in the path of a MergeError message, a longer path
	// is shortened to the errorPathHead lists at the head and the errorPathTail lists at the tail.
	maxErrorPath  = 8
	errorPathHead = 3
	errorPathTail = 4
)

// MergeError is the error of merging a list, e.g. "circular inclusion: top -> a -> [b -> c -> b]".
// Its message shortens a long path, use errors.As to get the full Path.
type MergeError struct {
	// Err is ErrCircularInclusion, or the error of looking up the last list in Path, which wraps
	// ErrListNotFound if the list is missing.
	Err error
	// Path is the names of the lists from the merged list to the failed one.
	Path []string
	// Cycle is the index in Path where the circle starts, whose list is included again by the last list.
	// It is -1 if the inclusion is not circular.
	Cycle int
}

func (e *MergeError) Error() string {
	switch {
	case errors.Is(e.Err, ErrCircularInclusion):
		return "circular inclusion: " + e.includeNestedPath()
	case errors.Is(e.Err, ErrListNotFound):
		return "list not found: " + e.includeNestedPath()
	default:
		return "lookup " + e.includeNestedPath() + ": " + e.Err.Error()
	}
}

func (e *MergeError) Unwrap() error {
	return e.Err
}

// includeNestedPath joins the names in Path by arrows with the circle in brackets, e.g. "top -> a -> [b -> c -> b]".
// The lists in the middle of a long path are omitted, e.g. "l0 -> l1 -> l2 -> ...(5 lists)... -> l8 -> l9 -> l10 -> l11".
func (e *MergeError) includeNestedPath() string {
	var b strings.Builder
	for i := 0; i < len(e.Path); i++ {
		if i > 0 {
			b.WriteString(" -> ")
		}
		if len(e.Path) > maxErrorPath && i == errorPathHead {
			omitted := len(e.Path) - errorPathHead - errorPathTail
			// The circle starts among the omitted lists.
			if e.Cycle >= i && e.Cycle < i+omitted {
				b.WriteByte('[')
			}
			fmt.Fprintf(&b, "...(%d lists)...", omitted)
			i += omitted - 1
			continue
		}
		if i == e.Cycle {
			b.WriteByte('[')
		}
		b.WriteString(e.Path[i])
	}
	if e.Cycle >= 0 {
		b.WriteByte(']')
	}
	return b.String()
}
