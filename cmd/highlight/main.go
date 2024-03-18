package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
)

func main() {
	var fileName string
	flag.StringVar(&fileName, "f", "", "File name")
	flag.Parse()

	if fileName == "" {
		fmt.Println("Error: File name is required.")
		os.Exit(1)
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	lexer := lexers.Match(fileName)
	style := styles.Get("monokai")
	formatter := html.New(html.Standalone(true), html.TabWidth(4), html.WithLineNumbers(true), html.LineNumbersInTable(true))

	iter, err := lexer.Tokenise(nil, string(data))
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iter)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	htmlBytes := bytes.ReplaceAll(buf.Bytes(), []byte("{{"), []byte("&#123;&#123;"))
	htmlBytes = bytes.ReplaceAll(htmlBytes, []byte("}}"), []byte("&#125;&#125;"))

	outputFileName := "output.html"
	err = os.WriteFile(outputFileName, htmlBytes, 0644)
	if err != nil
