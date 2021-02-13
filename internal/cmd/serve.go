package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-docs/internal/serve"
)

type serveCmd struct {
	commonCmd
}

const (
	menuTemplate = `
<div class="provider-docs-menu-title">Preview documentation</div>
<div class="provider-docs-menu-content">
  <ul class="provider-docs-menu-list menu-list">
    <div>
      <li class="menu-list-link"><a onclick="fetchContent('{{.Index.Path}}')" class="active">{{.Index.Name}}</a></li>
    </div>
{{ if .Resources }}
<div class="menu-list-category-wrapper expanded"><div>
      <li class="menu-list-category">
        <a class="menu-list-category-link">
          <i class="fa fa-angle-down"></i>
          <span class="menu-list-category-link-title">Resources</span>
        </a>
        <ul class="menu-list">
  {{range .Resources}}
<div><li class="menu-list-link">
            <a onclick="fetchContent('{{.Path}}')">
              {{.Name}}
            </a>
          </li></div>
  {{end}}
</ul></li></div></div>
{{ end }}
{{ if .Data }}
<div class="menu-list-category-wrapper expanded"><div>
      <li class="menu-list-category">
        <a class="menu-list-category-link">
          <i class="fa fa-angle-down"></i>
          <span class="menu-list-category-link-title">Data Sources</span>
        </a>
        <ul class="menu-list">
  {{range .Data}}
<div><li class="menu-list-link">
            <a onclick="fetchContent('{{.Path}}')">
              {{.Name}}
            </a>
          </li></div>
  {{end}}
</ul></li></div></div>
{{ end }}
  </ul>
</div>
`
)

func (cmd *serveCmd) Synopsis() string {
	return ""
}

func (cmd *serveCmd) Help() string {
	return `Usage: tfplugindocs serve`
}

func (cmd *serveCmd) Flags() *flag.FlagSet {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	return fs
}

func (cmd *serveCmd) Run(args []string) int {
	fs := cmd.Flags()
	err := fs.Parse(args)
	if err != nil {
		cmd.ui.Error(fmt.Sprintf("unable to parse flags: %s", err))
		return 1
	}

	return cmd.run(cmd.runInternal)
}

func (cmd *serveCmd) runInternal() error {
	cmd.ui.Info("Link to click - http://localhost:8080/tools/doc-preview")

	err := http.ListenAndServe("localhost:8080", http.HandlerFunc(handleServe))

	if err != nil {
		return fmt.Errorf("unable to validate website: %w", err)
	}

	return nil
}

func handleServe(w http.ResponseWriter, r *http.Request) {
	var err error
	if r.RequestURI == "/tools/doc-preview" {
		err = getDocPreviewPage(w)
	} else if r.RequestURI == "/markdown/menu" {
		err = getSidebarMenu(w)
	} else if strings.HasPrefix(r.RequestURI, "/markdown") {
		err = getMarkdownContent(w, r.RequestURI)
	} else {
		err = proxyRequest(w, r.RequestURI)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func getDocPreviewPage(w http.ResponseWriter) error {
	page, err := serve.FetchDocPreviewPage()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := io.Copy(w, page); err != nil {
		return err
	}

	return nil
}

func proxyRequest(w http.ResponseWriter, path string) error {
	get, err := http.Get(fmt.Sprintf("%s/%s", "https://registry.terraform.io", path))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", get.Header.Get("Content-Type"))
	w.WriteHeader(get.StatusCode)

	defer get.Body.Close()

	_, err = io.Copy(w, get.Body)
	if err != nil {
		return err
	}

	return nil
}

func getSidebarMenu(w http.ResponseWriter) error {
	menu, err := serve.GenerateMenu()
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

func getMarkdownContent(w http.ResponseWriter, path string) error {
	page, err := serve.ReadPage(strings.TrimPrefix(path, "/markdown/"))
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/javascript")
	if err := json.NewEncoder(w).Encode(page); err != nil {
		return err
	}

	return nil
}
