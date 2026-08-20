package confluence

import (
	"fmt"
	"strings"
)

// ── Balanced element scanning ─────────────────────────────────────────────────
//
// Confluence storage format nests freely: a TC table's "Screen Capture" cell can
// hold an expand macro containing another table. A non-greedy regex like
// `<table>.*?</table>` stops at the FIRST closing tag and so cuts such an element in
// half, producing markup with unclosed <ac:structured-macro> tags that Confluence
// rejects with "Unexpected EOF; was expecting a close tag".
//
// These helpers walk the raw markup counting depth, so element boundaries are always
// balanced, while leaving every byte we don't touch exactly as Confluence wrote it.

type tagToken struct {
	start, end int // byte range of the tag itself
	name       string
	closing    bool
	selfClose  bool
}

// scanTags returns every element tag in s, skipping comments, CDATA and declarations
// (macro parameters are routinely wrapped in CDATA that may contain "<" or ">").
func scanTags(s string) []tagToken {
	var out []tagToken
	i := 0
	for i < len(s) {
		rel := strings.IndexByte(s[i:], '<')
		if rel < 0 {
			break
		}
		p := i + rel

		switch {
		case strings.HasPrefix(s[p:], "<!--"):
			i = skipPast(s, p, "-->")
			continue
		case strings.HasPrefix(s[p:], "<![CDATA["):
			i = skipPast(s, p, "]]>")
			continue
		case strings.HasPrefix(s[p:], "<!"), strings.HasPrefix(s[p:], "<?"):
			i = skipPast(s, p, ">")
			continue
		}

		j := p + 1
		closing := false
		if j < len(s) && s[j] == '/' {
			closing = true
			j++
		}
		nameStart := j
		for j < len(s) && isNameByte(s[j]) {
			j++
		}
		name := strings.ToLower(s[nameStart:j])
		if name == "" {
			i = p + 1 // a bare "<" in text
			continue
		}

		// Find the closing ">", ignoring any inside quoted attribute values.
		var quote byte
		for j < len(s) {
			c := s[j]
			switch {
			case quote != 0:
				if c == quote {
					quote = 0
				}
			case c == '"' || c == '\'':
				quote = c
			case c == '>':
				goto done
			}
			j++
		}
		break // unterminated tag: stop scanning
	done:
		out = append(out, tagToken{
			start:     p,
			end:       j + 1,
			name:      name,
			closing:   closing,
			selfClose: s[j-1] == '/',
		})
		i = j + 1
	}
	return out
}

func skipPast(s string, from int, delim string) int {
	if idx := strings.Index(s[from:], delim); idx >= 0 {
		return from + idx + len(delim)
	}
	return len(s)
}

func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == ':' || c == '-' || c == '_' || c == '.'
}

// element is one balanced element: its full extent and its inner content.
type element struct {
	start, end           int
	innerStart, innerEnd int
}

// topLevelElements returns the balanced elements named in names that are not nested
// inside another element from that same set. Scanning a <tr> for "td"/"th" therefore
// skips the cells of any table nested within a cell.
func topLevelElements(s string, names ...string) []element {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}

	var (
		out   []element
		cur   element
		depth int
	)
	for _, t := range scanTags(s) {
		if !want[t.name] {
			continue
		}
		switch {
		case t.selfClose:
			if depth == 0 {
				out = append(out, element{start: t.start, end: t.end, innerStart: t.end, innerEnd: t.end})
			}
		case t.closing:
			if depth == 0 {
				continue // stray close tag
			}
			depth--
			if depth == 0 {
				cur.innerEnd = t.start
				cur.end = t.end
				out = append(out, cur)
			}
		default:
			if depth == 0 {
				cur = element{start: t.start, innerStart: t.end}
			}
			depth++
		}
	}
	return out
}

// ── Markup validation ─────────────────────────────────────────────────────────

// balancedElements are the container elements whose nesting must stay intact for
// Confluence to accept a page. Void/self-closing tags are deliberately excluded.
var balancedElements = map[string]bool{
	"table": true, "tbody": true, "thead": true, "tr": true, "td": true, "th": true,
	"ac:structured-macro": true, "ac:rich-text-body": true, "ac:plain-text-body": true,
	"ac:parameter": true, "ac:image": true,
}

// ValidateStorageBalance reports the first unbalanced container element in storage
// markup. Confluence rejects the whole PUT with a 400 when one is left open, so this
// catches a bad edit locally instead of after it has been sent.
func ValidateStorageBalance(storageHTML string) error {
	var stack []string
	for _, t := range scanTags(storageHTML) {
		if !balancedElements[t.name] || t.selfClose {
			continue
		}
		if !t.closing {
			stack = append(stack, t.name)
			continue
		}
		if len(stack) == 0 {
			return fmt.Errorf("unexpected closing tag </%s>", t.name)
		}
		if top := stack[len(stack)-1]; top != t.name {
			return fmt.Errorf("closing tag </%s> does not match open <%s>", t.name, top)
		}
		stack = stack[:len(stack)-1]
	}
	if len(stack) > 0 {
		return fmt.Errorf("unclosed element <%s>", stack[len(stack)-1])
	}
	return nil
}
