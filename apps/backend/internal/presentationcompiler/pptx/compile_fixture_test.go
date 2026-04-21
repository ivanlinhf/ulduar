package pptx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanlin/ulduar/apps/backend/internal/presentationdialect"
)

type compileFixture struct {
	Document              presentationdialect.Document `json:"document"`
	ExpectedEntries       map[string][]string          `json:"expectedEntries"`
	ExpectedBinaryEntries []string                     `json:"expectedBinaryEntries"`
}

func TestCompileWithAssetsMatchesPresetFixtures(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("testdata/preset-fixtures/*.json")
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no preset fixture files found")
	}

	for _, path := range paths {
		path := path
		t.Run(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), func(t *testing.T) {
			t.Parallel()

			var fixture compileFixture
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", path, err)
			}
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
			}

			compiled, err := CompileWithAssets(fixture.Document, buildFixtureCompileAssets(t, fixture.Document))
			if err != nil {
				t.Fatalf("CompileWithAssets() error = %v", err)
			}

			entries := readZIPEntries(t, compiled)
			for entryName, needles := range fixture.ExpectedEntries {
				content, ok := entries[entryName]
				if !ok {
					t.Fatalf("missing expected PPTX entry %q", entryName)
				}
				for _, needle := range needles {
					assertContains(t, content, needle)
				}
			}

			binaryEntries := readZIPBinaryEntries(t, compiled)
			for _, entryName := range fixture.ExpectedBinaryEntries {
				if _, ok := binaryEntries[entryName]; !ok {
					t.Fatalf("missing expected binary PPTX entry %q", entryName)
				}
			}
		})
	}
}

func buildFixtureCompileAssets(t *testing.T, document presentationdialect.Document) map[string]CompileAsset {
	t.Helper()

	refs := make(map[string]struct{})
	for _, slide := range document.Slides {
		for _, block := range slide.Blocks {
			if assetRef := dereferenceString(block.AssetRef); assetRef != "" {
				refs[assetRef] = struct{}{}
			}
		}
		for _, column := range slide.Columns {
			for _, block := range column.Blocks {
				if assetRef := dereferenceString(block.AssetRef); assetRef != "" {
					refs[assetRef] = struct{}{}
				}
			}
		}
	}

	assets := make(map[string]CompileAsset, len(refs))
	for assetRef := range refs {
		switch {
		case strings.HasPrefix(assetRef, "attachment:"):
			assets[assetRef] = CompileAsset{
				Filename:  strings.TrimPrefix(assetRef, "attachment:") + ".png",
				MediaType: "image/png",
				Data:      testPNGData(),
			}
		case strings.HasPrefix(assetRef, "theme:"):
			presetID := dereferenceString(document.ThemePresetID)
			themeKey := strings.TrimPrefix(assetRef, "theme:")
			themeAsset, ok := presentationdialect.LookupThemeBundleAsset(presetID, themeKey)
			if !ok {
				t.Fatalf("missing bundled theme asset for %q", assetRef)
			}
			assets[assetRef] = CompileAsset{
				Filename:  themeAsset.Filename,
				MediaType: themeAsset.MediaType,
				Data:      themeAsset.Data,
			}
		default:
			t.Fatalf("unsupported fixture asset ref %q", assetRef)
		}
	}
	return assets
}
