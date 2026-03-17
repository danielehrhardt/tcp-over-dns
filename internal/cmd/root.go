package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
)

var (
	cfgFile    string
	verbose    bool
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

// SetVersionInfo sets version information from build flags.
func SetVersionInfo(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

var rootCmd = &cobra.Command{
	Use:   "tcpdns",
	Short: "TCP over DNS — tunnel your traffic through DNS queries",
	Long: `tcpdns makes it dead simple to tunnel TCP traffic through DNS queries.

Perfect for bypassing captive portals (airplane WiFi, hotels, coffee shops)
when DNS queries are allowed but regular traffic is blocked.

Quick Start:
  1. tcpdns config init          # Set up your configuration
  2. tcpdns server setup         # Set up your VPS (run on server)
  3. tcpdns client connect       # Connect from your machine
  4. tcpdns proxy start          # Start SOCKS5 proxy

Documentation: https://github.com/danielehrhardt/tcp-over-dns`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Banner(appVersion)
		fmt.Printf("  Version:  %s\n", appVersion)
		fmt.Printf("  Commit:   %s\n", appCommit)
		fmt.Printf("  Built:    %s\n", appDate)
		fmt.Println()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.tcpdns/config.yml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(clientCmd)
	rootCmd.AddCommand(proxyCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(diagnoseCmd)

	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.SetHelpTemplate(fmt.Sprintf(`%s{{.Long}}%s

%sUsage:%s
  {{.UseLine}}

%sAvailable Commands:%s{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}

%sFlags:%s
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

%sGlobal Flags:%s
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`, ui.Dim, ui.Reset, ui.Bold, ui.Reset, ui.Bold, ui.Reset, ui.Bold, ui.Reset, ui.Bold, ui.Reset))
}

func exitWithError(msg string) {
	ui.Error(msg)
	os.Exit(1)
}
