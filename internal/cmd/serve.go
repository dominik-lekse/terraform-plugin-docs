package cmd

import (
	"flag"
	"fmt"
	"github.com/hashicorp/terraform-plugin-docs/internal/serve"
	"net/http"
	"os"
	"path/filepath"
)

type serveCmd struct {
	commonCmd

	addr string

	flagProviderName         string
	flagRenderedProviderName string
}

func (cmd *serveCmd) Synopsis() string {
	return "serve a generated plugin website from markdown in the current directory using the doc preview tool at https://registry.terraform.io/tools/doc-preview"
}

func (cmd *serveCmd) Help() string {
	return `Usage: tfplugindocs serve --addr [host:port]`
}

func (cmd *serveCmd) Flags() *flag.FlagSet {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.StringVar(&cmd.addr, "addr", "localhost:8080", "listen address")
	fs.StringVar(&cmd.flagProviderName, "provider-name", "", "provider name, as used in Terraform configurations")
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
	cmd.ui.Info(fmt.Sprintf("Preview docs at http://%s/tools/doc-preview", cmd.addr))

	providerName := cmd.flagProviderName
	if providerName == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		providerName = filepath.Base(wd)
	}

	handler := serve.NewHandler(providerName)
	err := http.ListenAndServe(cmd.addr, handler)

	if err != nil {
		return fmt.Errorf("unable to validate website: %w", err)
	}

	return nil
}
