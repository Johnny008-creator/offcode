package tools

import (
	"os"
	"strings"
	"testing"
)

func writeTempHTML(t *testing.T, html string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "browsertest-*.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(html); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestRenderedTextStaticContent(t *testing.T) {
	path := writeTempHTML(t, `<html><body><p>Hello from offcode browser test</p></body></html>`)
	text, err := RenderedText(path)
	if err != nil {
		t.Fatalf("RenderedText: %v", err)
	}
	if !strings.Contains(text, "Hello from offcode browser test") {
		t.Errorf("expected rendered text to contain the paragraph, got %q", text)
	}
}

func TestRenderedTextJavaScriptContent(t *testing.T) {
	// The whole point of a real browser (vs. fetch_url's static HTML
	// strip) is seeing content that JS inserts after load.
	path := writeTempHTML(t, `<html><body><div id="out"></div>
<script>document.getElementById('out').textContent = 'injected by JS';</script>
</body></html>`)
	text, err := RenderedText(path)
	if err != nil {
		t.Fatalf("RenderedText: %v", err)
	}
	if !strings.Contains(text, "injected by JS") {
		t.Errorf("expected JS-inserted text to be present, got %q", text)
	}
}

func TestPreviewPageReturnsJPEG(t *testing.T) {
	// JPEG (not PNG) specifically because a full-resolution PNG
	// screenshot base64-encoded into a vision model's context caused a
	// real "failed to tokenize prompt" backend error in live testing —
	// this is a regression test for that fix.
	path := writeTempHTML(t, `<html><body style="background:red;width:100px;height:100px"></body></html>`)
	img, err := PreviewPage(path, 200, 150)
	if err != nil {
		t.Fatalf("PreviewPage: %v", err)
	}
	if len(img) < 50 {
		t.Fatalf("screenshot suspiciously small: %d bytes", len(img))
	}
	if img[0] != 0xFF || img[1] != 0xD8 || img[2] != 0xFF {
		t.Errorf("expected JPEG magic bytes (FF D8 FF), got %v", img[:8])
	}
}

func TestPreviewPageDefaultSizeIsReasonable(t *testing.T) {
	// A real-world-ish page at the default viewport shouldn't produce a
	// multi-megabyte payload — that's exactly what broke the vision
	// model's tokenizer before switching to JPEG + a smaller default
	// viewport.
	path := writeTempHTML(t, `<html><body style="margin:0"><div style="background:linear-gradient(45deg,#1e3c78,#ff5050);width:100%;height:100vh"></div></body></html>`)
	img, err := PreviewPage(path, 0, 0) // 0,0 -> defaults
	if err != nil {
		t.Fatalf("PreviewPage: %v", err)
	}
	const maxReasonableBytes = 500_000
	if len(img) > maxReasonableBytes {
		t.Errorf("default-viewport screenshot is %d bytes, expected under %d", len(img), maxReasonableBytes)
	}
}
