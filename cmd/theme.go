package cmd

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// bannerMode controls how the startup banner is rendered.
type bannerMode int

const (
	bannerFlame   bannerMode = iota // ember→gold→green gradient (default)
	bannerClassic                   // solid green, no gradient
	bannerPlain                     // monochrome gray
)

// resolveBannerMode returns the active banner rendering mode.
// Resolution order: CREX_BANNER env → config banner_style → flame.
func resolveBannerMode() bannerMode {
	if v := os.Getenv("CREX_BANNER"); v != "" {
		return parseBannerMode(v)
	}
	if cfg != nil && cfg.BannerStyle != "" {
		return parseBannerMode(cfg.BannerStyle)
	}
	return bannerFlame
}

func parseBannerMode(s string) bannerMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "classic":
		return bannerClassic
	case "plain":
		return bannerPlain
	default:
		return bannerFlame
	}
}

// theme holds the resolved color palette for the current terminal background.
type theme struct {
	dark    bool
	flame   []lipgloss.Color // wing gradient: ember tips → gold → green bridge
	green   lipgloss.Color   // CREX text and command names
	dim     lipgloss.Color   // secondary text
	tagline lipgloss.Style   // "Terminal workspace manager for ..."
	accent  lipgloss.Style   // "resurrected." — the phoenix word
	version lipgloss.Style   // build version line
}

// detectTheme probes the terminal background and returns the matching palette.
// Resolution order: CREX_THEME env → lipgloss OSC 11 query → default dark.
func detectTheme() theme {
	if isDark() {
		return darkTheme()
	}
	return lightTheme()
}

// isDark checks whether the terminal has a dark background.
// CREX_THEME=light|dark overrides auto-detection (useful for tmux users
// where OSC 11 passthrough may be blocked).
func isDark() bool {
	switch strings.ToLower(os.Getenv("CREX_THEME")) {
	case "light":
		return false
	case "dark":
		return true
	default:
		return lipgloss.HasDarkBackground()
	}
}

// -- Dark theme --------------------------------------------------------------

// flameDark: deep ember → amber → chartreuse bridge into green.
// 9 steps for smooth gradient across ~10 visible wing characters per line.
var flameDark = []lipgloss.Color{
	"#8B1A00", // 1  deep ember
	"#BF3600", // 2  hot ember
	"#E05800", // 3  deep orange
	"#F27D00", // 4  true orange
	"#FFA200", // 5  amber
	"#FFC107", // 6  warm gold
	"#E8D44D", // 7  gold → yellow-green
	"#C8E650", // 8  chartreuse bridge
	"#8FFF6B", // 9  lime bridge (near CREX green)
}

func darkTheme() theme {
	return theme{
		dark:  true,
		flame: flameDark,
		green: lipgloss.Color("#5FFF87"),
		dim:   lipgloss.Color("#8C8C8C"),
		tagline: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			Italic(true),
		accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA200")).
			Italic(true).
			Bold(true),
		version: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#707070")),
	}
}

// -- Light theme -------------------------------------------------------------

// flameLight: dark crimson → burnt ochre → olive bridge into forest green.
// Higher chroma, lower lightness — contrast on white backgrounds.
var flameLight = []lipgloss.Color{
	"#6B0F00", // 1  dark crimson
	"#9A2200", // 2  brick red
	"#C04400", // 3  burnt orange
	"#D96A00", // 4  deep orange
	"#C48500", // 5  dark amber
	"#A69000", // 6  deep ochre
	"#7D9A00", // 7  olive-gold
	"#4E8C0A", // 8  dark chartreuse
	"#2D7A20", // 9  forest-lime bridge
}

func lightTheme() theme {
	return theme{
		dark:  false,
		flame: flameLight,
		green: lipgloss.Color("#1A8A3E"),
		dim:   lipgloss.Color("#6C6C6C"),
		tagline: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true),
		accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C04400")).
			Italic(true).
			Bold(true),
		version: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")),
	}
}
