package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extAst "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

type Head struct {
	Title string `xml:"title"`
}

type Body struct {
	Outlines []Outline `xml:"outline"`
}

type Outline struct {
	Text   string `xml:"text,attr"`
	Type   string `xml:"type,attr"`
	XMLUrl string `xml:"xmlUrl,attr"`
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	res := []byte{}
	_, err = resp.Body.Read(res)
	return res, err
}

func main() {
	// https://github.com/weekend-project-space/top-rss-list/blob/main/README.md
	data, err := os.ReadFile("README.md")
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(
			extension.Table,
		),
	)

	reader := text.NewReader(data)
	doc := md.Parser().Parse(reader)

	var feeds []Outline

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Kind() == extAst.KindTable {
			table := n.(*extAst.Table)
			var rows [][]string
			for row := table.FirstChild(); row != nil; row = row.NextSibling() {
				if row.Kind() != extAst.KindTableRow {
					continue
				}
				var columns []string
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					if cell.Kind() == extAst.KindTableCell {
						var buf bytes.Buffer
						if cell.FirstChild() != nil && cell.FirstChild().Kind() == ast.KindLink {
							link, ok := cell.FirstChild().(*ast.Link)
							if ok {
								columns = append(columns, string(link.Destination))
								continue
							}
						}
						for n := 0; n < cell.Lines().Len(); n++ {
							segment := cell.Lines().At(n)
							buf.Write(segment.Value(reader.Source()))
						}
						columns = append(columns, strings.TrimSpace(buf.String()))
					}
				}
				rows = append(rows, columns)
			}
			for _, row := range rows[1:] { // Skip header row
				if len(row) >= 2 {
					name := row[0]
					url := row[1]
					feeds = append(feeds, Outline{Text: name, Type: "rss", XMLUrl: url})
				}
			}
		}
		return ast.WalkContinue, nil
	})

	// Create OPML structure
	opml := OPML{
		Version: "2.0",
		Head:    Head{Title: "RSS Feeds"},
		Body:    Body{Outlines: feeds},
	}

	// Generate OPML file
	outputFile, err := os.Create("feeds.opml")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer outputFile.Close()

	encoder := xml.NewEncoder(outputFile)
	encoder.Indent("", "  ")
	if err := encoder.Encode(opml); err != nil {
		fmt.Println("Error encoding XML:", err)
		return
	}
	fmt.Println("OPML file generated: feeds.opml")
}
