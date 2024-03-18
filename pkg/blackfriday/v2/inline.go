package blackfriday

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
)

// Functions to parse inline elements.

type Markdown struct {
	nesting      int
	maxNesting  int
	insideLink  bool
	extensions  ExtensionFlags
	notes       []*reference
	inlineCallback map[byte]func([]byte, int) (int, *Node)
}

func (p *Markdown) inline(currBlock *Node, data []byte) {
	p.nesting++
	beg, end := 0, 0
	for end < len(data) {
		handler := p.inlineCallback[data[end]]
		if handler != nil {
			consumed, node := handler(p, data, end)
			if consumed == 0 {
				end++
			} else {
				currBlock.AppendChild(data[beg:end])
				if node != nil {
					currBlock.AppendChild(node)
				}
				beg = end + consumed
				end = beg
			}
		} else {
			end++
		}
	}
	if beg < len(data) {
		currBlock.AppendChild(data[beg:end])
	}
	p.nesting--
}

func censored(p *Markdown, data []byte, offset int) (int, *Node) {
	c := data[offset]
	if p.extensions&NoIntraEmphasis != 0 {
		if !(offset+1 == len(data) || isspace(data[offset+1]) || ispunct(data[offset+1])) {
			return 0, nil
		}
	}
	emph := NewNode(Censored)
	p.inline(emph, data[:offset])
	return offset + 1, emph
}

func emphasis(p *Markdown, data []byte, offset int) (int, *Node) {
	c := data[offset]
	if p.extensions&NoIntraEmphasis != 0 {
		if !(offset+1 == len(data) || isspace(data[offset+1]) || ispunct(data[offset+1])) {
			return 0, nil
		}
	}
	emph := NewNode(Emph)
	p.inline(emph, data[:offset])
	return offset + 1, emph
}

func doubleEmphasis(p *Markdown, data []byte, offset int) (int, *Node) {
	c := data[offset]
	for i := offset; i < len(data); i++ {
		if data[i] == c && data[i-1] != '\\' && !isspace(data[i-1]) {
			nodeType := Strong
			if c == '~' {
				nodeType = Del
			}
			node := NewNode(nodeType)
			p.inline(node, data[:i])
			return i + 1, node
		}
	}
	return 0, nil
}

func tripleEmphasis(p *Markdown, data []byte, offset int, c byte) (int, *Node) {
	for i := offset; i < len(data); i++ {
		if data[i] == c && data[i-1] != '\\' && !isspace(data[i-1]) {
			switch {
			case i+2 < len(data) && data[i+1] == c && data[i+2] == c:
				strong := NewNode(Strong)
				em := NewNode(Emph)
				strong.AppendChild(em)
				p.inline(em, data[:i])
				return i + 3, strong
			case i+1 < len(data) && data[i+1] == c:
				length, node := emphasis(p, data, i+1)
				if length == 0 {
					return 0, nil
				}
				return length, node
			default:
				length, node := doubleEmphasis(p, data, i+1)
				if length == 0 {
					return 0, nil
				}
				return length, node
			}
		}
	}
	return 0, nil
}

func codeSpan(p *Markdown, data []byte, offset int) (int, *Node) {
	nb := 0
	for i := offset; i < len(data); i++ {
		if data[i] == '`' {
			nb++
		} else if nb > 0 {
			break
		}
	}
	i := 0
	for end := offset; end < len(data); end++ {
		if data[end] == '`' && i == nb {
			code := NewNode(Code)
			code.Literal = data[offset+1 : end]
			return end + 1, code
		}
		if data[end] == '`' {
			i++
		} else {
			i = 0
		}
	}
	return 0, nil
}

func maybeLineBreak(p *Markdown, data []byte, offset int) (int, *Node) {
	if data[offset] == '\n' {
		return 1, NewNode(Hardbreak)
	}
	return 0, nil
}

func lineBreak(p *Markdown, data []byte, offset int) (int, *Node) {
	if p.extensions&HardLineBreak != 0 {
		return 1, NewNode(Hardbreak)
	}
	return 0, nil

