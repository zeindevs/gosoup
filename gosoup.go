package gosoup

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

type ErrorType int

const (
	ErrUnableToParse ErrorType = iota
	ErrNodeElementEmpty
	ErrElementNotFound
	ErrNoNextSibling
	ErrNoPreviousSibling
	ErrNoNextElementSibling
	ErrNoPreviousElementSibling
)

type Error struct {
	Type ErrorType
	msg  string
}

func (e Error) Error() string {
	return e.msg
}

func newError(err ErrorType, msg string) Error {
	return Error{Type: err, msg: msg}
}

func newErrorAttrs(err ErrorType, args []string) Error {
	return Error{Type: err, msg: fmt.Sprintf("element `%s` with attributes `%s` not found", args[0], strings.Join(args[1:], "="))}
}

var (
	errNodeElementEmpty = newError(ErrNodeElementEmpty, fmt.Sprintf("node element empty"))
)

type Root struct {
	Node  *html.Node
	Value string
	Error error
}

func HTMLParse(s string) (*Root, error) {
	h, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return nil, newError(ErrUnableToParse, "unable to parse the HTML")
	}

	for h.Type != html.ElementNode {
		switch h.Type {
		case html.DocumentNode:
			h = h.FirstChild
		case html.DoctypeNode:
			h = h.NextSibling
		case html.CommentNode:
			h = h.NextSibling
		}
	}

	return &Root{Node: h, Value: h.Data}, nil
}

func (r Root) Find(args ...string) *Root {
	if r.Node == nil {
		return &Root{Error: errNodeElementEmpty}
	}

	n, ok := findOne(r.Node, args, false, false)
	if !ok {
		return &Root{Error: newErrorAttrs(ErrElementNotFound, args)}
	}

	return &Root{Node: n, Value: n.Data}
}

func (r Root) FindAll(args ...string) ([]*Root, error) {
	if r.Node == nil {
		return nil, errNodeElementEmpty
	}

	n, ok := findAll(r.Node, args, false)
	if len(n) == 0 && !ok {
		return nil, newErrorAttrs(ErrElementNotFound, args)
	}

	roots := make([]*Root, 0, len(n))
	for i := 0; i < len(n); i++ {
		roots = append(roots, &Root{Node: n[i], Value: n[i].Data})
	}

	return roots, nil
}

func (r Root) FindStrict(args ...string) *Root {
	if r.Node == nil {
		return &Root{Error: errNodeElementEmpty}
	}

	n, ok := findOne(r.Node, args, false, false)
	if !ok {
		return &Root{Error: newErrorAttrs(ErrElementNotFound, args)}
	}

	return &Root{Node: n, Value: n.Data}
}

func (r Root) FindAllStrict(args ...string) ([]*Root, error) {
	if r.Node == nil {
		return nil, errNodeElementEmpty
	}

	n, ok := findAll(r.Node, args, true)
	if !ok && len(n) == 0 {
		return nil, newErrorAttrs(ErrElementNotFound, args)
	}

	roots := make([]*Root, 0, len(n))
	for i := 0; i < len(n); i++ {
		roots = append(roots, &Root{Node: n[i], Value: n[i].Data})
	}

	return roots, nil
}

func (r Root) FindNextSibling() *Root {
	if r.Node == nil {
		return &Root{Error: errNodeElementEmpty}
	}

	ns := r.Node.NextSibling
	if ns == nil {
		return &Root{Error: newError(ErrNoNextSibling, "no next sibling found")}
	}

	return &Root{Node: ns, Value: ns.Data}
}

func (r Root) FindPrevSibling() *Root {
	if r.Node == nil {
		return &Root{Error: errNodeElementEmpty}
	}

	ps := r.Node.PrevSibling
	if ps == nil {
		return &Root{Error: newError(ErrNoPreviousSibling, "no previous sibling found")}
	}

	return &Root{Node: ps, Value: ps.Data}
}

func (r Root) FindNextElementSibling() *Root {
	if r.Node == nil {
		return &Root{Error: errNodeElementEmpty}
	}

	ns := r.Node.NextSibling
	if ns == nil {
		return &Root{Error: newError(ErrNoNextElementSibling, "no next element sibling found")}
	}

	if ns.Type == html.ElementNode {
		return &Root{Node: ns, Value: ns.Data}
	}

	p := Root{Node: ns, Value: ns.Data}

	return p.FindNextElementSibling()
}

func (r Root) FindPrevElementSibling() *Root {
	if r.Node == nil {
		return &Root{Error: errNodeElementEmpty}
	}

	ps := r.Node.PrevSibling
	if ps == nil {
		return &Root{Error: newError(ErrNoPreviousElementSibling, "no previous element sibling found")}
	}

	if ps.Type == html.ElementNode {
		return &Root{Node: ps, Value: ps.Data}
	}

	p := Root{Node: ps, Value: ps.Data}

	return p.FindPrevElementSibling()
}

func (r Root) Children() []*Root {
	var children []*Root

	child := r.Node.FirstChild
	for child != nil {
		children = append(children, &Root{Node: child, Value: child.Data})
		child = child.NextSibling
	}

	return children
}

func (r Root) Attrs() map[string]string {
	if r.Node.Type != html.ElementNode {
		return nil
	}

	if len(r.Node.Attr) == 0 {
		return nil
	}

	return getKeyValue(r.Node.Attr)
}

func (r Root) Text() string {
	k := r.Node.FirstChild

checkNode:
	if k != nil && k.Type != html.TextNode {
		k = k.NextSibling
		if k == nil {
			return ""
		}
		goto checkNode
	}

	if k != nil {
		r, _ := regexp.Compile(`^\s+$`)
		if ok := r.MatchString(k.Data); ok {
			k = k.NextSibling
			if k == nil {
				return ""
			}
			goto checkNode
		}
		return k.Data
	}

	return ""
}

func (r Root) HTML() string {
	var buf bytes.Buffer
	if err := html.Render(&buf, r.Node); err != nil {
		return ""
	}

	return buf.String()
}

func (r Root) FullText() string {
	var buf bytes.Buffer
	var f func(*html.Node)

	f = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		if n.Type == html.ElementNode {
			f(n.FirstChild)
		}
		if n.NextSibling != nil {
			f(n.NextSibling)
		}
	}

	f(r.Node.FirstChild)

	return buf.String()
}

func attributeAndValueEquals(attr html.Attribute, attribute, value string) bool {
	return attr.Key == attribute && attr.Val == value
}

func attributeContainsValue(attr html.Attribute, attribute, value string) bool {
	if attr.Key == attribute {
		for _, attrVal := range strings.Fields(attr.Val) {
			if attrVal == value {
				return true
			}
		}
	}

	return false
}

func matchElementName(n *html.Node, name string) bool {
	return name == "" || name == n.Data
}

func getKeyValue(attributes []html.Attribute) map[string]string {
	keyValues := make(map[string]string)
	for i := 0; i < len(attributes); i++ {
		_, exists := keyValues[attributes[i].Key]
		if exists == false {
			keyValues[attributes[i].Key] = attributes[i].Val
		}
	}

	return keyValues
}

func findOne(n *html.Node, args []string, uni, strict bool) (*html.Node, bool) {
	if uni {
		if n.Type == html.ElementNode && matchElementName(n, args[0]) {
			if len(args) > 1 && len(args) < 4 {
				for i := 0; i < len(n.Attr); i++ {
					attr := n.Attr[i]
					searchAttrName := args[1]
					searchAttrVal := args[2]
					if (strict && attributeAndValueEquals(attr, searchAttrName, searchAttrName)) ||
						(!strict && attributeContainsValue(attr, searchAttrName, searchAttrVal)) {
						return n, true
					}
				}
			} else if len(args) == 1 {
				return n, true
			}
		}
	}

	uni = true
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		nn, ok := findOne(c, args, true, strict)
		if ok {
			return nn, ok
		}
	}

	return nil, false
}

func findAll(n *html.Node, args []string, strict bool) ([]*html.Node, bool) {
	var f func(*html.Node, []string, bool)
	nodeLinks := make([]*html.Node, 0, 10)

	f = func(n *html.Node, args []string, uni bool) {
		if uni {
			if n.Type == html.ElementNode && matchElementName(n, args[0]) {
				if len(args) > 1 && len(args) < 4 {
					for i := 0; i < len(n.Attr); i++ {
						attr := n.Attr[i]
						searchAttrName := args[1]
						searchAttrVal := args[2]
						if (strict && attributeAndValueEquals(attr, searchAttrName, searchAttrVal)) ||
							(!strict && attributeContainsValue(attr, searchAttrName, searchAttrVal)) {
							nodeLinks = append(nodeLinks, n)
						}
					}
				} else if len(args) == 1 {
					nodeLinks = append(nodeLinks, n)
				}
			}
		}

		uni = true
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c, args, true)
		}
	}

	f(n, args, false)

	return nodeLinks, false
}
