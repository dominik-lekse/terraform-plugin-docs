package serve

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/adrg/frontmatter"
	"github.com/hashicorp/terraform-plugin-docs/internal/provider"
	"html/template"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	Guides    []*Page
	Data      []*Page
	Resources []*Page
}

type Page struct {
	Name              string `json:"name"`
	ProviderName      string `json:"providerName"`
	ShortProviderName string `json:"shortProviderName"`
	Title             string `json:"title"`
	Content           string `json:"content"`
	Path              string `json:"path"`
}

func NewHandler(providerName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.RequestURI == "/tools/doc-preview" {
			err = getDocPreviewPage(w)
		} else if r.RequestURI == "/markdown/menu" {
			err = getSidebarMenu(w, providerName)
		} else if strings.HasPrefix(r.RequestURI, "/markdown") {
			err = getMarkdownContent(w, r.RequestURI, providerName)
		} else {
			proxyRequest(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	})
}

func fetchDocPreviewPage() (io.Reader, error) {
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
					Data: serveJs,
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

func generateMenu(providerName string) (*Menu, error) {
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

		page, err := readPage(path, providerName)
		if err != nil {
			return err
		}

		if path == "docs/index.md" {
			menu.Index = page
		} else if strings.HasPrefix(path, "docs/guides/") {
			menu.Guides = append(menu.Guides, page)
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

func readPage(path string, providerName string) (*Page, error) {
	path = filepath.Clean(path)

	if !strings.HasSuffix(path, ".md") {
		return nil, errors.New("request for page should have markdown extension")
	}

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

	meta := map[string]string{}
	_, err = frontmatter.Parse(bytes.NewReader(content.Bytes()), &meta)
	if err != nil {
		return nil, err
	}

	pageTitle := ""
	if s, hasPageTitle := meta["page_title"]; hasPageTitle {
		pageTitle = s
	}

	_, name := filepath.Split(path)
	return &Page{
		Name:              name[:strings.Index(name, ".")],
		ProviderName:      providerName,
		ShortProviderName: provider.ProviderShortName(providerName),
		Title:             pageTitle,
		Content:           content.String(),
		Path:              path,
	}, nil
}

func extractLayout(content string) string {
	groups := layout.FindStringSubmatch(content)
	if len(groups) != 2 {
		return "unknown"
	}

	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(groups[1]), `"`), `"`)
}

func getDocPreviewPage(w http.ResponseWriter) error {
	page, err := fetchDocPreviewPage()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := io.Copy(w, page); err != nil {
		return err
	}

	return nil
}

func proxyRequest(w http.ResponseWriter, r *http.Request) {
	websiteUrl, err := url.Parse(website)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rp := &httputil.ReverseProxy{
		// Flush immediately after each write to the client
		FlushInterval: -1,
		Transport:     http.DefaultTransport,
		Director: func(req *http.Request) {
			req.Host = websiteUrl.Host
			req.URL.Scheme = websiteUrl.Scheme
			req.URL.Host = websiteUrl.Host
		},
	}

	rp.ServeHTTP(w, r)
}

func getSidebarMenu(w http.ResponseWriter, providerName string) error {
	menu, err := generateMenu(providerName)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html")

	t, err := template.New("menu").Parse(menuTemplate)
	if err != nil {
		return err
	}
	err = t.Execute(w, menu)
	if err != nil {
		return err
	}
	return nil
}

func getMarkdownContent(w http.ResponseWriter, path string, providerName string) error {
	page, err := readPage(strings.TrimPrefix(path, "/markdown/"), providerName)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/javascript")
	if err := json.NewEncoder(w).Encode(page); err != nil {
		return err
	}

	return nil
}
