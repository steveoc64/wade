package main

import (
	"fmt"
	"strings"

	"github.com/gowade/html"
)

var (
	nilCode = &codeNode{
		typ:  NakedCodeNode,
		code: "nil",
	}
)

func textNodeCode(text string) []*codeNode {
	parts := parseTextMustache(text)
	ret := make([]*codeNode, len(parts))

	for i, part := range parts {
		var cn *codeNode
		if part.isMustache {
			cn = ncn(fmt.Sprintf("fmt.Sprint(%v)", part.content))
		} else {
			cn = &codeNode{
				typ:  StringCodeNode,
				code: part.content,
			}
		}

		ret[i] = &codeNode{
			typ:      FuncCallCodeNode,
			code:     CreateTextNodeOpener,
			children: []*codeNode{cn},
		}
	}

	return ret
}

func attributeValueCode(parts []textPart) string {
	fmtStr := ""
	mustaches := []string{}
	for _, part := range parts {
		if part.isMustache {
			fmtStr += "%v"
			mustaches = append(mustaches, part.content)
		} else {
			fmtStr += part.content
		}
	}

	mStr := strings.Join(mustaches, ", ")
	return fmt.Sprintf("fmt.Sprintf(`%v`, %v)", fmtStr, mStr)
}

func mapFieldAssignmentCode(field string, value string) string {
	return fmt.Sprintf(`"%v": %v`, field, value)
}

func elementAttrsCode(attrs []html.Attribute) *codeNode {
	if len(attrs) == 0 {
		return nilCode
	}

	assignments := make([]*codeNode, len(attrs))
	for i, attr := range attrs {
		valueCode := attributeValueCode(parseTextMustache(attr.Val))
		assignments[i] = &codeNode{
			typ:  NakedCodeNode,
			code: mapFieldAssignmentCode(attr.Key, valueCode),
		}
	}

	return &codeNode{
		typ:      CompositeCodeNode,
		code:     AttributeMapOpener,
		children: assignments,
	}
}

func chAppend(a *[]*codeNode, b []*codeNode) {
	for _, item := range b {
		if item != nil {
			*a = append(*a, item)
		}
	}
}

// Filter out text nodes that are just garbage space-only text node (i.e "\n\t\t\t\t")
// generated by the html parser
//
// Remove those nodes entirely for those that are at the beginning and the end of
// parent elements. Turn others into just a space.
func filterTextStrings(list []*codeNode) []*codeNode {
	for i, item := range list {
		if item.typ == FuncCallCodeNode && len(item.children) == 1 {
			s := item.children[0]
			if s.typ == StringCodeNode && strings.TrimSpace(s.code) == "" {
				if i == 0 || i == len(list)-1 {
					list[i] = nil
				} else {
					s.code = " "
				}
			}
		}
	}

	ret := make([]*codeNode, 0, len(list))
	for _, item := range list {
		if item != nil {
			ret = append(ret, item)
		}
	}

	return ret
}

func (cpl *HtmlCompiler) genChildren(node *html.Node, vda *varDeclArea) []*codeNode {
	children := make([]*codeNode, 0)
	i := 0
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		chAppend(&children, cpl.generateRec(c, vda))

		i++
	}

	return filterTextStrings(children)
}

func NewHtmlCompiler() *HtmlCompiler {
	return &HtmlCompiler{[]error{}}
}

type HtmlCompiler struct {
	errors []error
}

func (c *HtmlCompiler) Error() (s string) {
	for _, e := range c.errors {
		s += e.Error() + "\n"
	}
	return
}

func (c *HtmlCompiler) elementCode(node *html.Node, vda *varDeclArea) (*codeNode, error) {
	switch node.Data {
	case "for":
		return c.forLoopCode(node, vda)
	case "if":
		return c.ifControlCode(node, vda)
	}

	children := c.genChildren(node, vda)
	childrenCode := nilCode
	if len(children) != 0 {
		childrenCode = &codeNode{
			typ:      ElemListCodeNode,
			code:     "",
			children: children,
		}
	}

	return &codeNode{
		typ:  FuncCallCodeNode,
		code: CreateElementOpener,
		children: []*codeNode{
			&codeNode{typ: StringCodeNode, code: node.Data}, // element tag name
			elementAttrsCode(node.Attr),
			childrenCode,
		},
	}, nil
}

func (c *HtmlCompiler) generateRec(node *html.Node, vda *varDeclArea) []*codeNode {
	if node.Type == html.TextNode {
		return textNodeCode(node.Data)
	}

	if node.Type == html.ElementNode {
		cn, err := c.elementCode(node, vda)
		if err != nil {
			c.errors = append(c.errors, err)
		}

		return []*codeNode{cn}
	}

	return nil
}

func (c *HtmlCompiler) generate(node *html.Node) *codeNode {
	vda := newVarDeclArea()

	ret := &codeNode{
		typ:  BlockCodeNode,
		code: RenderFuncOpener,
		children: []*codeNode{
			vda.codeNode,
			ncn("return "),
			c.generateRec(node, vda)[0],
		},
	}

	vda.saveToCN()
	return ret
}
