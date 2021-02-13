package serve

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

const (
	website = "https://registry.terraform.io/tools/doc-preview"
)

var layout = regexp.MustCompile("(?ims)---\\n(?:.*\\n)?layout:([^\\n]*).*---\\n")

type Menu struct {
	Index     *Page
	Data      []*Page
	Resources []*Page
}

type Page struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Path    string `json:"path"`
}

func FetchDocPreviewPage() (io.Reader, error) {
	get, err := http.Get(website)
	if err != nil {
		return nil, err
	}

	defer get.Body.Close()

	body, err := injectScriptIntoPage(get.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func injectScriptIntoPage(body io.Reader) (io.Reader, error) {
	parse, err := html.Parse(body)
	if err != nil {
		return nil, err
	}

	var crawl func(n *html.Node)
	crawl = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			js := &html.Node{
				Type: html.ElementNode,
				Data: "script",
				FirstChild: &html.Node{
					// TODO this content should come from the js file
					Data: `
let fetchContent = (u) => {
    fetch("/markdown/" + u)
        .then(r => r.json())
        .then(r => {
            document.getElementById("ember21").value = r.content // TODO shouldn't bake in the ID
            document.getElementById("ember21").dispatchEvent(new Event("input", {"bubbles": true}))
        })

    document.getElementById("ember21").style.display = "none"
}

let updateMenu = (n) => {
    fetch("/markdown/menu")
        .then(r => r.text())
        .then(r => {
            n.innerHTML = r
        })
}

let textArea = new MutationObserver((mutations, ob) => {
    mutations.forEach((mutation) => {
        if (!mutation.addedNodes) return

        for (let i = 0; i < mutation.addedNodes.length; i++) {
            // do things to your newly added nodes here
            let node = mutation.addedNodes[i]
            if (node.nodeName === "TEXTAREA") { // TODO is this smart enough?
                fetchContent("docs/index.md") // TODO this shouldn't be hard coded
                ob.disconnect()
            }
        }
    })
})
let menu = new MutationObserver((mutations, ob) => {
    mutations.forEach((mutation) => {
        if (!mutation.addedNodes) return

        for (let i = 0; i < mutation.addedNodes.length; i++) {
            // do things to your newly added nodes here
            let node = mutation.addedNodes[i]
            if (node.nodeName === "DIV" && node.getAttribute("class") === "provider-docs-menu") {
                updateMenu(node)
                ob.disconnect()
            }
        }
    })
})

textArea.observe(document.body, {
    childList: true
    , subtree: true
    , attributes: false
    , characterData: false
})
menu.observe(document.body, {
    childList: true
    , subtree: true
    , attributes: false
    , characterData: false
})
`,
					Type: html.TextNode,
				},
			}
			n.AppendChild(js)
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			crawl(child)
		}
	}

	crawl(parse)

	var buffer bytes.Buffer
	if err := html.Render(&buffer, parse); err != nil {
		return nil, err
	}

	return &buffer, nil
}

func GenerateMenu() (*Menu, error) {
	menu := &Menu{}
	err := filepath.Walk("./docs", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		page, err := ReadPage(path)
		if err != nil {
			return err
		}

		if path == "docs/index.md" {
			page.Name = extractLayout(page.Content)
			menu.Index = page
		} else if strings.HasPrefix(path, "docs/resources/") {
			menu.Resources = append(menu.Resources, page)
		} else if strings.HasPrefix(path, "docs/data-sources/") {
			menu.Data = append(menu.Data, page)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return menu, nil
}

// TODO ensure file is of correct type
func ReadPage(path string) (*Page, error) {
	path = filepath.Clean(path)

	if !strings.HasPrefix(path, "docs") {
		return nil, errors.New("request for page outside of docs folder")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	content := &bytes.Buffer{}
	_, err = io.Copy(content, file)
	if err != nil {
		return nil, err
	}

	_, name := filepath.Split(path)
	return &Page{
		Name:    name[:strings.Index(name, ".")],
		Content: content.String(),
		Path:    path,
	}, nil
}

func extractLayout(content string) string {
	groups := layout.FindStringSubmatch(content)
	if len(groups) != 2 {
		return "unknown"
	}

	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(groups[1]), `"`), `"`)
}
