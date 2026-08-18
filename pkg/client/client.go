package client

import (
	"context"
	"fmt"

	"github.com/jardon/zpkg-build/pkg/builder"
	"github.com/jardon/zpkg-build/pkg/manifest"
)

type Options struct {
	OutputDir     string
	ExportFormat  string
	NoArchive     bool
	KeepContainer bool
}

type Client struct {
	builder         *builder.Builder
	manifest        *manifest.RecipeManifest
	rawRecipe       map[string]interface{}
	recipeHash      string
	reproducibility manifest.Reproducibility
}

func New(manifestPath string, opts Options) (*Client, error) {
	m, recipeHash, _, err := manifest.LoadAndHydrateManifest(manifestPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	rawRecipe, err := manifest.LoadManifestRaw(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load raw manifest: %w", err)
	}

	b, err := builder.New(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}
	applyOpts(b, opts)

	return &Client{
		builder:         b,
		manifest:        m,
		rawRecipe:       rawRecipe,
		recipeHash:      recipeHash,
		reproducibility: manifest.AnalyzeReproducibility(rawRecipe),
	}, nil
}

func NewWithManifest(baseDir string, m *manifest.RecipeManifest, rawRecipe map[string]interface{}, recipeHash string, opts Options) (*Client, error) {
	if baseDir == "" {
		baseDir = "."
	}
	b, err := builder.NewFromManifest(baseDir, m, rawRecipe, recipeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}
	applyOpts(b, opts)

	return &Client{
		builder:         b,
		manifest:        m,
		rawRecipe:       rawRecipe,
		recipeHash:      recipeHash,
		reproducibility: manifest.AnalyzeReproducibility(rawRecipe),
	}, nil
}

func (c *Client) Pull(ctx context.Context) error {
	return c.builder.Pull(ctx)
}

func (c *Client) Build(ctx context.Context) error {
	return c.builder.Build(ctx)
}

func (c *Client) Package(ctx context.Context) error {
	return c.builder.Package(ctx)
}

func (c *Client) Export(ctx context.Context) error {
	return c.builder.Export(ctx)
}

func (c *Client) Clean(step string) error {
	return c.builder.Clean(step)
}

func (c *Client) Destroy() error {
	return c.builder.Destroy()
}

func (c *Client) Manifest() *manifest.RecipeManifest {
	return c.manifest
}

func (c *Client) RecipeHash() string {
	return c.recipeHash
}

func (c *Client) Reproducibility() manifest.Reproducibility {
	return c.reproducibility
}

func applyOpts(b *builder.Builder, opts Options) {
	if opts.OutputDir != "" {
		b.SetOutputDir(opts.OutputDir)
	}
	if opts.ExportFormat != "" {
		b.SetExportFormat(opts.ExportFormat)
	}
	if opts.NoArchive {
		b.SetNoArchive(true)
	}
	if opts.KeepContainer {
		b.SetKeepContainer(true)
	}
}
