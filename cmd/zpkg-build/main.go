package main

import (
	"fmt"
	"os"

	"github.com/jardon/zpkg-build/pkg/builder"
	"github.com/jardon/zpkg-build/pkg/manifest"
	"github.com/spf13/cobra"
)

var manifestFile string
var outputDir string
var noArchive bool
var exportFormat string
var keepContainer bool

func main() {
	rootCmd := &cobra.Command{
		Use:   "zpkg-build",
		Short: "A containerized & isolated software packaging CLI",
		Long:  "zpkg-build compiles and packages software inside strictly isolated sandbox environments.",
	}

	rootCmd.PersistentFlags().StringVarP(&manifestFile, "file", "f", "package.yaml", "Path to package.yaml manifest")
	rootCmd.PersistentFlags().BoolVar(&keepContainer, "keep", false, "Keep the build environment alive after the build for debugging")

	rootCmd.AddCommand(pullCmd())
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(packageCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(cleanCmd())
	rootCmd.AddCommand(destroyCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(analyzeCmd())
	rootCmd.AddCommand(hashCmd())

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
			b.SetKeepContainer(keepContainer)
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
			b.SetKeepContainer(keepContainer)
			return b.Package(cmd.Context())
		},
	}
}

func exportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Archive build output and export to host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if noArchive && outputDir == "." {
				return fmt.Errorf("--no-archive requires --output-dir to be set")
			}
			b, err := newBuilder()
			if err != nil {
				return err
			}
			b.SetOutputDir(outputDir)
			b.SetNoArchive(noArchive)
			b.SetExportFormat(exportFormat)
			b.SetKeepContainer(keepContainer)
			return b.Export(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Output directory for exported archives")
	cmd.Flags().BoolVar(&noArchive, "no-archive", false, "Skip archiving and place files directly in output directory")
	cmd.Flags().StringVar(&exportFormat, "format", "zpkg", "Export format: zpkg, tar.gz, tar, tar.xz, zip")
	return cmd
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

func destroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the build environment kept alive by a previous --keep build",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := newBuilder()
			if err != nil {
				return err
			}
			return b.Destroy()
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

func analyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Check manifest reproducibility",
		RunE: func(cmd *cobra.Command, args []string) error {
			rawRecipe, err := manifest.LoadManifestRaw(manifestFile)
			if err != nil {
				return err
			}

			r := manifest.AnalyzeReproducibility(rawRecipe)

			if r.Deterministic {
				fmt.Println("✓ deterministic")
				return nil
			}

			fmt.Println("✗ non-deterministic")
			for _, w := range r.Warnings {
				fmt.Printf("  - %s\n", w)
			}
			os.Exit(1)
			return nil
		},
	}
}

func hashCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hash",
		Short: "Print canonical SHA-256 of the manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			hash, err := manifest.ComputeRecipeHash(manifestFile)
			if err != nil {
				return err
			}
			fmt.Println(hash)
			return nil
		},
	}
}
