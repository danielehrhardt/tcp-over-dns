package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
	"github.com/danielehrhardt/tcp-over-dns/internal/web"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Open the graphical dashboard in your browser",
	Long: `Starts a local web server and opens a graphical dashboard
in your default browser.

The dashboard provides:
  - One-click connect/disconnect
  - Live connection status
  - SOCKS proxy toggle
  - System diagnostics
  - Configuration editor
  - Activity log

The web UI only listens on localhost and is not accessible from the network.`,
	RunE: runUI,
}

func init() {
	uiCmd.Flags().IntP("port", "p", 7654, "port for the web UI")
	uiCmd.Flags().Bool("no-open", false, "don't auto-open the browser")
	rootCmd.AddCommand(uiCmd)
}

func runUI(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	noOpen, _ := cmd.Flags().GetBool("no-open")

	ui.Banner(appVersion)
	fmt.Println()
	ui.Info("Starting graphical dashboard...")
	ui.Info("URL: http://127.0.0.1:%d", port)
	ui.Info("Press Ctrl+C to stop")
	fmt.Println()

	return web.Start(port, !noOpen)
}
