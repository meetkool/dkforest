package database

import (
	"dkforest/pkg/bfchroma"
	bf "dkforest/pkg/blackfriday/v2"
	"github.com/alecthomas/chroma/formatters/html"
	html2 "html"
	"io"
	"regexp"
	"strings"
)

func MyRenderer(db *DkfDB, withLineNumbers, lineNumbersInTable bool) *Renderer {
	// Defines the HTML rendering flags that are used
	var flags = bf.UseXHTML | bf.SkipImages

	r := &Renderer{
		DB: db,
		Base: bfchroma.NewRenderer(
			bfchroma.WithoutAutodetect(),
			bfchroma.ChromaOptions(
				html.WithLineNumbers(withLineNumbers),
				html.LineNumbersInTable(lineNumbersInTable),
			),
			bfchroma.Extend(
				bf.NewHTMLRenderer(bf.HTMLRendererParameters{
					Flags: flags,
				}),
			),
		),
	}
	return r
}

func MyRendererForum(db *DkfDB, withLineNumbers, lineNumbersInTable bool) *Renderer {
	// Defines the HTML rendering flags that are used
	var flags = bf.UseXHTML

	r := &Renderer{
		DB: db,
		Base: bfchroma.NewRenderer(
			bfchroma.WithoutAutodetect(),
			bfchroma.ChromaOptions(
				html.WithLineNumbers(withLineNumbers),
				html.LineNumbersInTable(lineNumbersInTable),
			),
			bfchroma.Extend(
				bf.NewHTMLRenderer(bf.HTMLRendererParameters{
					Flags: flags,
				}),
			),
		),
	}
	return r
}

type Renderer struct {
	DB   *DkfDB
	Base *bfchroma.Renderer
}

var roomNameF = `\w{3,50}`
var roomTagRgx = regexp.MustCompile(`#(` + roomNameF + `)`)

func (r Renderer) RenderNode(w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
	switch node.Type {
	case bf.Text:
		if node.Parent.Type != bf.Link {
			node.Literal = []byte(html2.UnescapeString(string(node.Literal)))

			if roomTagRgx.MatchString(string(node.Literal)) {
				node.Literal = []byte(roomTagRgx.ReplaceAllStringFunc(string(node.Literal), func(s string) string {
					if room, err := r.DB.GetChatRoomByName(strings.TrimPrefix(s, "#")); err == nil {
						return `<a href="/chat/` + room.Name + `" target="_top">` + s + `</a>`
					}
					return s
				}))
				_, _ = w.Write(node.Literal)
				return bf.GoToNext
			}
		}
	case bf.Code:
		node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
	case bf.CodeBlock:
		node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
	default:
	}
	return r.Base.RenderNode(w, node, entering)
}

func (r Renderer) RenderHeader(w io.Writer, ast *bf.Node) {
	r.Base.RenderHeader(w, ast)
}

func (r Renderer) RenderFooter(w io.Writer, ast *bf.Node) {
	r.Base.RenderFooter(w, ast)
}
