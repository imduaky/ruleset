package domainlist

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"unsafe"
)

type MetaValue struct {
	Name string
	Meta []string
}

func (v MetaValue) String() string {
	sb := strings.Builder{}
	ll := len(v.Name)
	for _, vmeta := range v.Meta {
		// ' ' + '@' + vmeta
		ll += 1 + 1 + len(vmeta)
	}

	sb.Grow(ll)
	sb.WriteString(v.Name)
	for _, vmeta := range v.Meta {
		sb.WriteByte(' ')
		sb.WriteByte(Meta)
		sb.WriteString(vmeta)
	}

	return sb.String()
}

type DomainList struct {
	Includes []MetaValue

	Domain  []MetaValue
	Suffix  []MetaValue
	Keyword []MetaValue
	Regex   []MetaValue
}

func (l DomainList) MarshalText() ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	buffer.Grow(
		len(prefixFullWith)*len(l.Domain) +
			len(prefixIncludeWith)*len(l.Includes) +
			len(prefixKeywordWith)*len(l.Keyword) +
			len(prefixRegexpWith)*len(l.Regex) +
			// Most domain names are typically 5 characters long, with one LF (\n) to the tail.
			(len(l.Keyword)+len(l.Regex)+len(l.Includes)+len(l.Domain)+len(l.Suffix))*(5+1),
	)

	if len(l.Includes) > 0 {
		writeWithPrefix(buffer, prefixIncludeWith, l.Includes)
	}
	if len(l.Suffix) > 0 {
		writeLn(buffer)
		writeWithPrefix(buffer, "", l.Suffix)
	}
	if len(l.Keyword) > 0 {
		writeLn(buffer)
		writeWithPrefix(buffer, prefixKeywordWith, l.Keyword)
	}
	if len(l.Domain) > 0 {
		writeLn(buffer)
		writeWithPrefix(buffer, prefixFullWith, l.Domain)
	}
	if len(l.Regex) > 0 {
		writeLn(buffer)
		writeWithPrefix(buffer, prefixRegexpWith, l.Regex)
	}

	return buffer.Bytes(), nil
}

func writeWithPrefix(sw *bytes.Buffer, prefix string, meta []MetaValue) {
	writePrefix := len(prefix) > 0

	for i := 0; i < len(meta); i++ {
		m := meta[i]
		if writePrefix {
			sw.WriteString(prefix)
		}
		sw.WriteString(m.String())
		writeLn(sw)
	}
}

func writeLn(bw *bytes.Buffer) {
	bw.WriteByte('\n') // LF
}

func (l *DomainList) UnmarshalText(data []byte) error {
	lineCount := 0

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lineCount++
		line := scanner.Bytes()
		commentStart := bytes.IndexByte(line, Comment)
		if commentStart <= 0 {
			// all line is a comment
			continue
		}

		metaStart := max(len(line), bytes.IndexByte(line, Meta))

		mainBody := bytes.TrimSpace(line[:min(commentStart, metaStart)])
		if len(mainBody) == 0 {
			if metaStart != -1 {
				return fmt.Errorf("there was a meta but the content is empty at line: %d: meta at: %d",
					lineCount, metaStart)
			}

			// empty
			continue
		}

		// there was a meta
		if commentStart >= metaStart {

		}

		switch {
		case bytes.HasPrefix(line, unsafe.Slice(unsafe.StringData(prefixIncludeWith), len(prefixIncludeWith))):
			// include

		case bytes.HasPrefix(line, unsafe.Slice(unsafe.StringData(prefixFullWith), len(prefixFullWith))):
			// full

		case bytes.HasPrefix(line, unsafe.Slice(unsafe.StringData(prefixKeywordWith), len(prefixKeywordWith))):
			// keyword

		case bytes.HasPrefix(line, unsafe.Slice(unsafe.StringData(prefixRegexpWith), len(prefixRegexpWith))):
			// regexp

		default:
			// suffix

		}

	}
}
