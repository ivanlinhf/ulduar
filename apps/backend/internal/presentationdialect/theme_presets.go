package presentationdialect

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
)

const themeBundleAssetHeroImage = "hero-image"

type ThemePresetPalette struct {
	Background  string
	Surface     string
	SurfaceAlt  string
	Text        string
	Muted       string
	Accent      string
	AccentAlt   string
	Success     string
	Warning     string
	InverseText string
	Outline     string
}

type ThemePresetFonts struct {
	Latin string
	CJK   string
}

type ThemePresetTreatments struct {
	Cover   string
	Chapter string
	Cards   string
	Table   string
	Image   string
}

type ThemePresetSpacing struct {
	Tight int
	Base  int
	Loose int
}

type ThemeBundleAsset struct {
	Filename  string
	MediaType string
	Data      []byte
}

type ThemePresetDefinition struct {
	Metadata   ThemePresetMetadata
	Palette    ThemePresetPalette
	Fonts      ThemePresetFonts
	Treatments ThemePresetTreatments
	Spacing    ThemePresetSpacing
	Assets     map[string]ThemeBundleAsset
}

type presetHeroSpec struct {
	// Colors are six-character RGB hex strings used to synthesize a lightweight
	// bundled hero image for preset-owned `theme:hero-image` slots.
	background string
	surface    string
	accent     string
	accentAlt  string
	outline    string
}

var builtInThemePresetDefinitions = []ThemePresetDefinition{
	{
		Metadata: ThemePresetMetadata{
			ID:          ThemePresetGeneralClean,
			Label:       "General Clean",
			Description: "Default balanced preset for general-purpose decks.",
			IsDefault:   true,
		},
		Palette: ThemePresetPalette{
			Background:  "FFFFFF",
			Surface:     "F8FAFC",
			SurfaceAlt:  "E2E8F0",
			Text:        "1F2937",
			Muted:       "64748B",
			Accent:      "2563EB",
			AccentAlt:   "0F766E",
			Success:     "059669",
			Warning:     "D97706",
			InverseText: "FFFFFF",
			Outline:     "CBD5E1",
		},
		Fonts: ThemePresetFonts{
			Latin: "Arial",
			CJK:   "Noto Sans CJK JP",
		},
		Treatments: ThemePresetTreatments{
			Cover:   "Balanced clean hero with cool gradient header band and quiet headline block.",
			Chapter: "Neutral chapter divider with crisp accent stripe and reserved supporting image slot.",
			Cards:   "Soft dashboard cards with cool-gray surfaces, blue emphasis, and modest corner rhythm.",
			Table:   "Flat informational table with subtle row fills and clear rule contrast.",
			Image:   "Straight crops with clean borders and restrained shadow treatment.",
		},
		Spacing: ThemePresetSpacing{
			Tight: 16,
			Base:  24,
			Loose: 36,
		},
		Assets: map[string]ThemeBundleAsset{
			themeBundleAssetHeroImage: {
				Filename:  "general-clean-hero-image.png",
				MediaType: "image/png",
				Data: mustBuildThemeHeroPNG(presetHeroSpec{
					background: "FFFFFF",
					surface:    "E2E8F0",
					accent:     "2563EB",
					accentAlt:  "0F766E",
					outline:    "CBD5E1",
				}),
			},
		},
	},
	{
		Metadata: ThemePresetMetadata{
			ID:          ThemePresetTravelEditorial,
			Label:       "Travel Editorial",
			Description: "Editorial preset for image-led travel and itinerary narratives.",
		},
		Palette: ThemePresetPalette{
			Background:  "F7F1EA",
			Surface:     "FFF9F3",
			SurfaceAlt:  "E6D5C3",
			Text:        "2D241D",
			Muted:       "7A6859",
			Accent:      "A45C40",
			AccentAlt:   "2F5D7C",
			Success:     "4A7C59",
			Warning:     "C0843D",
			InverseText: "FFF9F3",
			Outline:     "D9C9B7",
		},
		Fonts: ThemePresetFonts{
			Latin: "Georgia",
			CJK:   "Noto Serif CJK JP",
		},
		Treatments: ThemePresetTreatments{
			Cover:   "Image-led editorial cover with warm paper overlay, serif headlines, and travel kicker accents.",
			Chapter: "Panoramic chapter divider with label chip, image band, and spacious vertical rhythm.",
			Cards:   "Magazine-style comparison cards with warm surfaces and image-first emphasis.",
			Table:   "Editorial summary panel with warm ruled table styling and muted support copy.",
			Image:   "Large travel-image crops with caption-led framing and gentle edge shading.",
		},
		Spacing: ThemePresetSpacing{
			Tight: 18,
			Base:  28,
			Loose: 42,
		},
		Assets: map[string]ThemeBundleAsset{
			themeBundleAssetHeroImage: {
				Filename:  "travel-editorial-hero-image.png",
				MediaType: "image/png",
				Data: mustBuildThemeHeroPNG(presetHeroSpec{
					background: "F7F1EA",
					surface:    "E6D5C3",
					accent:     "A45C40",
					accentAlt:  "2F5D7C",
					outline:    "D9C9B7",
				}),
			},
		},
	},
}

func BuiltInThemePresets() []ThemePresetMetadata {
	presets := make([]ThemePresetMetadata, 0, len(builtInThemePresetDefinitions))
	for _, preset := range builtInThemePresetDefinitions {
		presets = append(presets, preset.Metadata)
	}
	return presets
}

func ResolveThemePresetID(requested string) string {
	requested = strings.TrimSpace(requested)
	for _, preset := range builtInThemePresetDefinitions {
		if preset.Metadata.ID == requested {
			return preset.Metadata.ID
		}
	}
	return ThemePresetGeneralClean
}

func ResolveThemePreset(requested string) ThemePresetDefinition {
	resolvedID := ResolveThemePresetID(requested)
	for _, preset := range builtInThemePresetDefinitions {
		if preset.Metadata.ID == resolvedID {
			return cloneThemePresetDefinition(preset)
		}
	}
	return cloneThemePresetDefinition(builtInThemePresetDefinitions[0])
}

func LookupThemeBundleAsset(requestedPresetID, key string) (ThemeBundleAsset, bool) {
	preset := ResolveThemePreset(requestedPresetID)
	asset, ok := preset.Assets[key]
	if !ok {
		return ThemeBundleAsset{}, false
	}
	return cloneThemeBundleAsset(asset), true
}

func cloneThemePresetDefinition(preset ThemePresetDefinition) ThemePresetDefinition {
	cloned := preset
	if preset.Assets != nil {
		cloned.Assets = make(map[string]ThemeBundleAsset, len(preset.Assets))
		for key, asset := range preset.Assets {
			cloned.Assets[key] = cloneThemeBundleAsset(asset)
		}
	}
	return cloned
}

func cloneThemeBundleAsset(asset ThemeBundleAsset) ThemeBundleAsset {
	cloned := asset
	cloned.Data = append([]byte(nil), asset.Data...)
	return cloned
}

// mustBuildThemeHeroPNG synthesizes a deterministic 320x180 PNG used for
// preset-owned bundled hero art. The drawing logic intentionally stays simple:
// a full-canvas gradient base, one diagonal accent band, and two outlined panel
// blocks arranged in slide-like coordinates so local dev and container builds
// always ship owned preset imagery without external asset files.
func mustBuildThemeHeroPNG(spec presetHeroSpec) []byte {
	const (
		width  = 320
		height = 180
	)

	background := mustHexColor(spec.background)
	surface := mustHexColor(spec.surface)
	accent := mustHexColor(spec.accent)
	accentAlt := mustHexColor(spec.accentAlt)
	outline := mustHexColor(spec.outline)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		v := float64(y) / float64(height-1)
		left := blendColor(background, surface, 0.18+0.42*v)
		right := blendColor(left, accentAlt, 0.12)
		for x := 0; x < width; x++ {
			h := float64(x) / float64(width-1)
			img.SetRGBA(x, y, blendColor(left, right, h*0.55))
		}
	}

	fillRect(img, 0, 0, width, 26, blendColor(surface, outline, 0.45))
	fillRect(img, 0, height-44, width, 44, blendColor(background, surface, 0.72))
	fillRect(img, 20, 34, 140, 12, blendColor(accent, outline, 0.35))
	fillRect(img, 20, 54, 90, 8, blendColor(outline, surface, 0.2))
	fillRect(img, 20, 68, 70, 8, blendColor(outline, surface, 0.35))
	fillRect(img, 184, 28, 112, 72, blendColor(surface, background, 0.15))
	fillRect(img, 194, 40, 92, 12, blendColor(accentAlt, surface, 0.4))
	fillRect(img, 194, 58, 74, 8, blendColor(outline, surface, 0.3))
	fillRect(img, 194, 72, 62, 8, blendColor(outline, surface, 0.45))
	fillRect(img, 194, 112, 106, 50, blendColor(surface, outline, 0.25))
	fillRect(img, 204, 122, 72, 10, blendColor(accent, surface, 0.25))
	fillRect(img, 204, 138, 50, 8, blendColor(outline, surface, 0.45))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x-y > 58 && x-y < 88 {
				img.SetRGBA(x, y, blendColor(img.RGBAAt(x, y), accent, 0.32))
			}
		}
	}
	for y := 92; y < 126; y++ {
		startX := 48 + (y-92)*3
		endX := 192 + (y-92)*2
		if startX < 0 {
			startX = 0
		}
		if endX > width {
			endX = width
		}
		for x := startX; x < endX; x++ {
			img.SetRGBA(x, y, blendColor(img.RGBAAt(x, y), accentAlt, 0.18))
		}
	}
	drawBorder(img, 184, 28, 112, 72, outline)
	drawBorder(img, 194, 112, 106, 50, outline)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}

func fillRect(img *image.RGBA, x, y, width, height int, c color.RGBA) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			img.SetRGBA(x+dx, y+dy, c)
		}
	}
}

func drawBorder(img *image.RGBA, x, y, width, height int, c color.RGBA) {
	for dx := 0; dx < width; dx++ {
		img.SetRGBA(x+dx, y, c)
		img.SetRGBA(x+dx, y+height-1, c)
	}
	for dy := 0; dy < height; dy++ {
		img.SetRGBA(x, y+dy, c)
		img.SetRGBA(x+width-1, y+dy, c)
	}
}

func blendColor(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 0xFF,
	}
}

func mustHexColor(value string) color.RGBA {
	if len(value) != 6 {
		panic("invalid hex color length")
	}
	return color.RGBA{
		R: mustHexByte(value[0], value[1]),
		G: mustHexByte(value[2], value[3]),
		B: mustHexByte(value[4], value[5]),
		A: 0xFF,
	}
}

func mustHexByte(high, low byte) uint8 {
	return hexNibble(high)<<4 | hexNibble(low)
}

func hexNibble(value byte) uint8 {
	switch {
	case value >= '0' && value <= '9':
		return uint8(value - '0')
	case value >= 'a' && value <= 'f':
		return uint8(value-'a') + 10
	case value >= 'A' && value <= 'F':
		return uint8(value-'A') + 10
	default:
		panic("invalid hex digit")
	}
}
