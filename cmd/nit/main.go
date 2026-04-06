package main

import (
	"os"

	"github.com/joshuazd/nit/internal/cli"
	"github.com/joshuazd/nit/internal/ui"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	var args cli.Args

	rootCmd := &cobra.Command{
		Use:     "nit [target]",
		Short:   "Terminal diff viewer with inline review comments",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, positional []string) error {
			if len(positional) > 0 {
				args.FilePath, args.CommitRange = cli.ParseTarget(positional[0])
			}
			return ui.Run(args)
		},
	}

	flags := rootCmd.Flags()
	flags.StringVar(&args.Mode, "mode", "", "Diff mode: branch, unstaged, all")
	flags.StringVar(&args.PathFilter, "path", "", "Filter to specific file or directory path")
	flags.BoolVarP(&args.Verbose, "verbose", "v", false, "Enable verbose logging to stderr")
	flags.StringVar(&args.ExportComments, "export-comments", "", "Export comments on quit (- for stdout, or file path)")
	flags.StringVar(&args.ExportFormat, "export-format", "markdown", "Comment export format: markdown, json")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
