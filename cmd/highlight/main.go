package main

import (
	"bytes"
	"flag"
	"os"

	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
)

func main() {
	var fileName string
	flag.StringVar(&fileName, "f", "", "File name")
	flag.Parse()
	by, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	lexer := lexers.Match(fileName)
	style := styles.Get("monokai")
	formatter := html.New(html.Standalone(true), html.TabWidth(4), html.WithLineNumbers(true), html.LineNumbersInTable(true))
	iterator, _ := lexer.Tokenise(nil, string(by))
	buf := bytes.Buffer{}
	_ = formatter.Format(&buf, style, iterator)
	out := buf.Bytes()
	out = bytes.ReplaceAll(out, []byte("{{"), []byte("&#123;&#123;"))
	out = bytes.ReplaceAll(out, []byte("}}"), []byte("&#125;&#125;"))
	_ = os.WriteFile("output.html", out, 0644)
}
