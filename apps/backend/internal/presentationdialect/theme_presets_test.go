package presentationdialect

import (
	"bytes"
	"image"
	"strings"
	"testing"
)

func TestResolveThemePresetProvidesCuratedDefinitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id        string
		label     string
		isDefault bool
		accent    string
		latinFont string
		cjkFont   string
		heroAsset string
	}{
		{
			id:        ThemePresetGeneralClean,
			label:     "General Clean",
			isDefault: true,
			accent:    "2563EB",
			latinFont: "Arial",
			cjkFont:   "Noto Sans CJK JP",
			heroAsset: "general-clean-hero-image.png",
		},
		{
			id:        ThemePresetTravelEditorial,
			label:     "Travel Editorial",
			accent:    "A45C40",
			latinFont: "Georgia",
			cjkFont:   "Noto Serif CJK JP",
			heroAsset: "travel-editorial-hero-image.png",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.id, func(t *testing.T) {
			t.Parallel()

			preset := ResolveThemePreset(tt.id)
			if preset.Metadata.ID != tt.id {
				t.Fatalf("preset.Metadata.ID = %q, want %q", preset.Metadata.ID, tt.id)
			}
			if preset.Metadata.Label != tt.label {
				t.Fatalf("preset.Metadata.Label = %q, want %q", preset.Metadata.Label, tt.label)
			}
			if preset.Metadata.IsDefault != tt.isDefault {
				t.Fatalf("preset.Metadata.IsDefault = %t, want %t", preset.Metadata.IsDefault, tt.isDefault)
			}
			if preset.Palette.Accent != tt.accent {
				t.Fatalf("preset.Palette.Accent = %q, want %q", preset.Palette.Accent, tt.accent)
			}
			if preset.Fonts.Latin != tt.latinFont || preset.Fonts.CJK != tt.cjkFont {
				t.Fatalf("preset.Fonts = %#v, want Latin=%q CJK=%q", preset.Fonts, tt.latinFont, tt.cjkFont)
			}
			if preset.Treatments.Cover == "" || preset.Treatments.Chapter == "" || preset.Treatments.Cards == "" || preset.Treatments.Table == "" || preset.Treatments.Image == "" {
				t.Fatalf("preset.Treatments = %#v, want all preset treatments populated", preset.Treatments)
			}
			if preset.Spacing.Tight <= 0 || preset.Spacing.Base < preset.Spacing.Tight || preset.Spacing.Loose < preset.Spacing.Base {
				t.Fatalf("preset.Spacing = %#v, want ascending positive rhythm tokens", preset.Spacing)
			}

			asset, ok := preset.Assets[themeBundleAssetHeroImage]
			if !ok {
				t.Fatalf("preset.Assets missing %q", themeBundleAssetHeroImage)
			}
			if asset.Filename != tt.heroAsset {
				t.Fatalf("asset.Filename = %q, want %q", asset.Filename, tt.heroAsset)
			}
			if asset.MediaType != "image/png" {
				t.Fatalf("asset.MediaType = %q, want image/png", asset.MediaType)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(asset.Data))
			if err != nil {
				t.Fatalf("image.DecodeConfig() error = %v", err)
			}
			if format != "png" {
				t.Fatalf("theme asset format = %q, want png", format)
			}
			if cfg.Width != 320 || cfg.Height != 180 {
				t.Fatalf("theme asset size = %dx%d, want 320x180", cfg.Width, cfg.Height)
			}
		})
	}

	t.Run("distinct bundled hero art", func(t *testing.T) {
		t.Parallel()

		generalAsset, ok := ResolveThemePreset(ThemePresetGeneralClean).Assets[themeBundleAssetHeroImage]
		if !ok {
			t.Fatalf("general preset missing %q", themeBundleAssetHeroImage)
		}
		travelAsset, ok := ResolveThemePreset(ThemePresetTravelEditorial).Assets[themeBundleAssetHeroImage]
		if !ok {
			t.Fatalf("travel preset missing %q", themeBundleAssetHeroImage)
		}
		if bytes.Equal(generalAsset.Data, travelAsset.Data) {
			t.Fatal("expected preset hero assets to be distinct")
		}
	})

	t.Run("design resolver omits bundled assets", func(t *testing.T) {
		t.Parallel()

		preset := ResolveThemePresetDesign(ThemePresetTravelEditorial)
		if preset.Metadata.ID != ThemePresetTravelEditorial {
			t.Fatalf("preset.Metadata.ID = %q, want %q", preset.Metadata.ID, ThemePresetTravelEditorial)
		}
		if preset.Assets != nil {
			t.Fatalf("preset.Assets = %#v, want nil", preset.Assets)
		}
	})

	t.Run("resolved assets are cloned", func(t *testing.T) {
		t.Parallel()

		preset := ResolveThemePreset(ThemePresetGeneralClean)
		asset, ok := preset.Assets[themeBundleAssetHeroImage]
		if !ok {
			t.Fatalf("preset.Assets missing %q", themeBundleAssetHeroImage)
		}
		asset.Data[0] ^= 0xFF

		lookedUp, ok := LookupThemeBundleAsset(ThemePresetGeneralClean, themeBundleAssetHeroImage)
		if !ok {
			t.Fatalf("LookupThemeBundleAsset() missing %q", themeBundleAssetHeroImage)
		}
		if bytes.Equal(asset.Data, lookedUp.Data) {
			t.Fatal("expected bundled asset lookup to return isolated bytes")
		}
	})
}

func TestResolveThemePresetFallbackRemainsGeneralClean(t *testing.T) {
	t.Parallel()

	if got := ResolveThemePresetID(" missing "); got != ThemePresetGeneralClean {
		t.Fatalf("ResolveThemePresetID() = %q, want %q", got, ThemePresetGeneralClean)
	}

	builtIns := BuiltInThemePresets()
	if len(builtIns) != 2 {
		t.Fatalf("len(BuiltInThemePresets()) = %d, want 2", len(builtIns))
	}
	if builtIns[0].ID != ThemePresetGeneralClean || !builtIns[0].IsDefault {
		t.Fatalf("BuiltInThemePresets()[0] = %#v", builtIns[0])
	}
	if builtIns[1].ID != ThemePresetTravelEditorial {
		t.Fatalf("BuiltInThemePresets()[1] = %#v", builtIns[1])
	}
	travelPreset := ResolveThemePreset(ThemePresetTravelEditorial)
	if !strings.Contains(travelPreset.Treatments.Cover, "editorial") {
		t.Fatalf("travel editorial cover treatment = %q, want editorial guidance", travelPreset.Treatments.Cover)
	}
}
