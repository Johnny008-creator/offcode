package tui

import (
	"fmt"
	"os"

	"github.com/Johnny008-creator/offcode/internal/config"
)

var bannerIcon = [3]string{
	"▛▀▀▀▀▜",
	"▌ oc ▐",
	"▙▄▄▄▄▟",
}

func bannerInfoLines(cfg config.Config) [3]string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	return [3]string{
		fmt.Sprintf("offcode v%s", appVersion),
		fmt.Sprintf("%s · %s · mode=%s", modelHeaderLabel(cfg.Model), cfg.Backend, cfg.Mode),
		cwd,
	}
}

// renderBanner composes the icon and info lines side by side. When
// forTranscript is true it's styled to sit inline as the first transcript
// block (left aligned, no centering); the splash screen renders the same
// lines but centers the whole block on screen itself.
func renderBanner(cfg config.Config) string {
	info := bannerInfoLines(cfg)

	lineStyles := [3]func(string) string{
		func(s string) string { return headerStyle.Render(s) },
		func(s string) string { return subHeaderStyle.Render(s) },
		func(s string) string { return subHeaderStyle.Render(s) },
	}

	out := ""
	for i := 0; i < 3; i++ {
		icon := splashIconStyle.Render(bannerIcon[i])
		text := lineStyles[i](info[i])
		out += icon + "  " + text
		if i < 2 {
			out += "\n"
		}
	}
	return out
}
