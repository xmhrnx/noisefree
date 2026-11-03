package ui

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"
)

//go:embed fonts/*.ttf icons/*.svg
var assets embed.FS

// Brand colors: a soft gradient from coral (primary) to peach (accent), on
// neutral greys.
const (
	brandHot  = "#EE6F57"
	brandGold = "#F4A983"
)

var (
	darkPalette = giode.Palette{
		Primary: brandHot, Secondary: "#1E1F23", Background: "#0F1012", Surface: "#17181B",
		Text: "#ECECEE", Border: "#2A2B30", OnPrimary: "#FFFFFF", Muted: "#9A9BA3",
		Accent: brandGold, Success: "#4ADE80", Warning: "#FBBF24", Danger: "#F87171", Info: "#60A5FA",
	}
	lightPalette = giode.Palette{
		Primary: "#DE5D46", Secondary: "#F1F1F3", Background: "#F6F6F7", Surface: "#FFFFFF",
		Text: "#18181B", Border: "#E4E4E7", OnPrimary: "#FFFFFF", Muted: "#6B6B74",
		Accent: "#D98A5F", Success: "#16A34A", Warning: "#CA8A04", Danger: "#DC2626", Info: "#2563EB",
	}
)

// Shared tints, mixed once from theme tokens so they follow light/dark.
var (
	tintPrimary   = giode.Mix("theme.primary", "theme.surface", 0.14)
	tintPrimaryHi = giode.Mix("theme.primary", "theme.surface", 0.24)
	tintAccent    = giode.Mix("theme.accent", "theme.surface", 0.14)
	tintSuccess   = giode.Mix("theme.success", "theme.surface", 0.16)
	tintWarning   = giode.Mix("theme.warning", "theme.surface", 0.16)
	tintDanger    = giode.Mix("theme.danger", "theme.surface", 0.16)
	raised        = giode.Mix("theme.text", "theme.surface", 0.04)
	hover         = giode.Mix("theme.text", "theme.background", 0.06)
	track         = giode.Mix("theme.text", "theme.surface", 0.08)
	sidebarBg     = giode.Mix("theme.text", "theme.background", 0.02)
)

// Text styles.
var (
	h1       = giode.Styles{FontSize: 24, FontWeight: giode.FontWeightBold, Color: "theme.text"}
	h2       = giode.Styles{FontSize: 16, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}
	body     = giode.Styles{FontSize: 13.5, Color: "theme.text", LineHeight: 20}
	muted    = giode.Styles{FontSize: 13, Color: "theme.muted", LineHeight: 19}
	caption  = giode.Styles{FontSize: 11.5, Color: "theme.muted", FontWeight: giode.FontWeightMedium}
	overline = giode.Styles{FontSize: 11, Color: "theme.muted", FontWeight: giode.FontWeightSemiBold,
		TextTransform: giode.TextTransformUppercase}
)

// cardStyle is the surface every section sits on.
var cardStyle = giode.Styles{
	FlexDirection: giode.FlexDirectionColumn, Gap: 16, Padding: giode.UniformInset(20),
	Background: "theme.surface", BorderRadius: 16, BorderWidth: 1, BorderColor: "theme.border",
	BoxShadow: giode.BoxShadow{Y: 6, Blur: 24, Color: "#00000022"},
}

// fieldStyle is shared by selects and inputs.
var fieldStyle = giode.Styles{
	Background: raised, Color: "theme.text", BorderColor: "theme.border", BorderRadius: 10, Width: -1,
}

func primaryButton() giode.Styles {
	return giode.Styles{
		BackgroundGradient: giode.Linear(120, "theme.primary", "theme.accent"),
		Color:              "theme.onprimary", BorderRadius: 10, FontWeight: giode.FontWeightSemiBold,
		Padding: giode.SymmetricInset(16, 9), FontSize: 13,
	}
}

func secondaryButton() giode.Styles {
	return giode.Styles{
		Background: raised, Color: "theme.text", BorderRadius: 10, BorderWidth: 1, BorderColor: "theme.border",
		Padding: giode.SymmetricInset(14, 9), FontSize: 13, FontWeight: giode.FontWeightMedium,
	}
}

func icon(name string, size int, color string) giode.Element {
	return giode.Icon(name, giode.Styles{Width: properties.Width(size), Height: properties.Height(size), Color: color})
}

// SetupTheme registers Inter (with the Go fonts as fallback), the icons and
// the palettes.
func SetupTheme() error {
	faces, err := interFaces()
	if err != nil {
		return err
	}
	giode.RegisterFonts(append(faces, gofont.Collection()...))
	if err := giode.RegisterIconsFS(assets, "icons"); err != nil {
		return err
	}
	giode.SetPalette(giode.ModeDark, darkPalette)
	giode.SetPalette(giode.ModeLight, lightPalette)
	return nil
}

func interFaces() ([]font.FontFace, error) {
	weights := []struct {
		file   string
		weight font.Weight
	}{
		{"Inter-Regular.ttf", font.Normal}, {"Inter-Medium.ttf", font.Medium},
		{"Inter-SemiBold.ttf", font.SemiBold}, {"Inter-Bold.ttf", font.Bold},
	}
	var faces []font.FontFace
	for _, w := range weights {
		data, err := fs.ReadFile(assets, "fonts/"+w.file)
		if err != nil {
			return nil, err
		}
		face, err := opentype.Parse(bytes.Clone(data))
		if err != nil {
			return nil, fmt.Errorf("font %s: %w", w.file, err)
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: "Inter", Weight: w.weight}, Face: face})
	}
	return faces, nil
}
