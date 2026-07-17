package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/jardon/zpkg-build/pkg/builder"
)

var manifestFile string

func main() {
	rootCmd := &cobra.Command{
		Use:   "zpkg-build",
		Short: "A containerized & isolated software packaging CLI",
		Long:  "zpkg-build compiles and packages software inside strictly isolated sandbox environments.",
	}

	rootCmd.PersistentFlags().StringVarP(&manifestFile, "file", "f", "package.yaml", "Path to package.yaml manifest")

	rootCmd.AddCommand(pullCmd())
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(packageCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(cleanCmd())
	rootCmd.AddCommand(statusCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newBuilder() (*builder.Builder, error) {
	return builder.New(manifestFile)
}

func pullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Fetch project sources and verify patch checksums",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := newBuilder()
			if err != nil {
				return err
			}
			return b.Pull(cmd.Context())
		},
	}
}

func buildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Apply patches and compile source inside sandbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := newBuilder()
			if err != nil {
				return err
			}
			return b.Build(cmd.Context())
		},
	}
}

func packageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "package",
		Short: "Assemble package root and generate metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := newBuilder()
			if err != nil {
				return err
			}
			return b.Package(cmd.Context())
		},
	}
}

func exportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Archive build output and export to host",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := newBuilder()
			if err != nil {
				return err
			}
			return b.Export(cmd.Context())
		},
	}
}

func cleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean [step]",
		Short: "Purge workspace folders/state from specified stage forward",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := newBuilder()
			if err != nil {
				return err
			}
			return b.Clean(args[0])
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current cache metrics and active state steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := newBuilder()
			if err != nil {
				return err
			}
			return b.Status()
		},
	}
}
