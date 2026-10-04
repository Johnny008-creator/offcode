package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// browserTimeout bounds every headless-browser operation — a hung
// page load (bad network, an infinite JS loop) shouldn't be able to
// stall the whole agent loop indefinitely.
const browserTimeout = 20 * time.Second

// resolveTarget turns a local file path into a file:// URL for chromedp,
// leaving an actual http(s):// URL untouched.
func resolveTarget(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") || strings.HasPrefix(pathOrURL, "file://") {
		return pathOrURL, nil
	}
	abs, err := filepath.Abs(pathOrURL)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}
	return "file://" + abs, nil
}

func newBrowserContext(ctx context.Context) (context.Context, context.CancelFunc) {
	// --no-sandbox: running as a normal user inside what's often itself a
	// sandboxed/contained dev environment — chromium's own sandbox can
	// fail to initialize there. Headless, no GPU: this is a screenshot/
	// text-extraction tool, not an interactive session.
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	cancel := func() {
		browserCancel()
		allocCancel()
	}
	return browserCtx, cancel
}

// PreviewPage renders a local HTML file or URL in headless Chromium and
// returns a PNG screenshot — the standard way the agent gets to actually
// see UI output it built, instead of only reasoning about markup. Needs
// a vision-capable model to make use of the result (it's attached to the
// tool-result message as image data, same as view_image).
func PreviewPage(pathOrURL string, width, height int) ([]byte, error) {
	// Deliberately modest defaults, and JPEG rather than PNG: a
	// 1280x800 PNG screenshot base64-encoded into a vision model's
	// context is large enough to have caused a real "failed to
	// tokenize prompt" backend error in live testing — a vision model
	// judging a layout doesn't need full-resolution PNG fidelity.
	if width <= 0 {
		width = 1024
	}
	if height <= 0 {
		height = 640
	}
	target, err := resolveTarget(pathOrURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), browserTimeout)
	defer cancel()
	browserCtx, browserCancel := newBrowserContext(ctx)
	defer browserCancel()

	err = chromedp.Do(browserCtx,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate(target),
		chromedp.Sleep(300*time.Millisecond), // let CSS/fonts/layout settle
	)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", pathOrURL, err)
	}

	quality := int64(70)
	fromSurface := true
	screenshotJPEG := func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		r, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
			Format:      page.CaptureScreenshotFormatJpeg,
			Quality:     &quality,
			FromSurface: &fromSurface,
		})
		if err != nil {
			return nil, err
		}
		return r.Data, nil
	}
	buf, err := chromedp.Run(browserCtx, chromedp.Action[[]byte](screenshotJPEG))
	if err != nil {
		return nil, fmt.Errorf("screenshot %s: %w", pathOrURL, err)
	}
	return buf, nil
}

// RenderedText returns a page's visible text content after JavaScript
// has run (document.body.innerText) — fetch_url only strips tags from
// static HTML, so this is the complement for pages that render content
// client-side.
func RenderedText(pathOrURL string) (string, error) {
	target, err := resolveTarget(pathOrURL)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), browserTimeout)
	defer cancel()
	browserCtx, browserCancel := newBrowserContext(ctx)
	defer browserCancel()

	err = chromedp.Do(browserCtx,
		chromedp.Navigate(target),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		return "", fmt.Errorf("render %s: %w", pathOrURL, err)
	}
	text, err := chromedp.Run(browserCtx, chromedp.Evaluate[string](`document.body.innerText`))
	if err != nil {
		return "", fmt.Errorf("extract text from %s: %w", pathOrURL, err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("page rendered no visible text")
	}
	return text, nil
}
