package webserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/chromedp/chromedp"
)

// These tests use a real Chromium and the real server, but no model or user data.
// Run explicitly with: go test ./internal/webserver -run TestBrowser -v
func TestBrowserDraftsAndMobile(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Chromium")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var completions atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			w.Header().Set("Content-Type", "text/event-stream")
			if completions.Add(1) == 1 {
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"ask-1\",\"type\":\"function\",\"function\":{\"name\":\"ask_user\",\"arguments\":\"{\\\"question\\\":\\\"Choose a color\\\",\\\"options\\\":[\\\"Red\\\",\\\"Blue\\\"]}\"}}]}}]}\n\n")
			} else {
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Thanks for the custom answer.\"}}]}\n\n")
			}
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"browser-test"}]}`))
	}))
	defer backend.Close()
	s := New(config.Config{Backend: "lmstudio", BaseURL: backend.URL + "/v1", Model: "browser-test", Mode: "default"})
	server := httptest.NewServer(s.handler())
	defer server.Close()
	alloc, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.NoSandbox)...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(alloc)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	js := func(script string) chromedp.Action[chromedp.Void] {
		return chromedp.Func(func(ctx context.Context, target *chromedp.Target) error {
			_, err := chromedp.Evaluate[any](script)(ctx, target)
			return err
		})
	}
	poll := func(script string) chromedp.Action[chromedp.Void] {
		return chromedp.Func(func(ctx context.Context, target *chromedp.Target) error {
			_, err := chromedp.Poll[bool](script)(ctx, target)
			return err
		})
	}

	run := func(actions ...chromedp.Action[chromedp.Void]) {
		t.Helper()
		if err := chromedp.Do(ctx, actions...); err != nil {
			t.Fatal(err)
		}
	}
	check := func(expression string) {
		t.Helper()
		ok, err := chromedp.Run(ctx, chromedp.Evaluate[bool](expression))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal(expression)
		}
	}
	run(chromedp.Navigate(server.URL), chromedp.WaitVisible("#input"), poll(`!document.querySelector('#input').disabled && document.querySelector('#connectionStatus').hidden`))
	for _, width := range []int64{360, 390, 768, 1024, 1440} {
		run(chromedp.EmulateViewport(width, 800))
		check(`document.documentElement.scrollWidth <= innerWidth`)
		check(`document.querySelector('#composer').getBoundingClientRect().right <= innerWidth`)
	}
	run(chromedp.EmulateViewport(390, 844), js(`document.querySelector("#sbOpen").click()`), poll(`document.body.classList.contains('sb-mobile-open')`), js(`document.querySelector("#sbScrim").click()`))
	check(`!document.body.classList.contains('sb-mobile-open')`)
	// A rejected request must leave the user's message available for retry.
	run(js(`window.originalFetch = window.fetch; window.fetch = (url, opts) => url === '/api/message' ? Promise.resolve(new Response(JSON.stringify({error:'test rejection'}), {status:409})) : window.originalFetch(url, opts); document.querySelector('#input').value='keep this draft'; document.querySelector('#input').dispatchEvent(new Event('input')); document.querySelector('#composer').requestSubmit();`), poll(`document.querySelector('#toast').textContent === 'test rejection'`))
	check(`document.querySelector('#input').value === 'keep this draft'`)
	run(js(`window.fetch=window.originalFetch`))
	// A session change originating in a second tab must not send the old draft.
	oldID := s.sess.Snapshot().SessionID
	tab, closeTab := chromedp.NewContext(ctx)
	defer closeTab()
	if err := chromedp.Do(tab, chromedp.Navigate(server.URL), poll(`!document.querySelector('#input').disabled && document.querySelector('#connectionStatus').hidden`), js(`document.querySelector('#newChatBrand').click()`)); err != nil {
		t.Fatal(err)
	}
	run(poll(`document.querySelector('#input').value === ''`))
	if s.sess.Snapshot().SessionID == oldID {
		t.Fatal("second tab did not switch session")
	}
	// The question dialog accepts a custom answer, not just listed options.
	if !s.sess.Submit("Ask me which color to use") {
		t.Fatal("could not start question")
	}
	run(poll(`document.querySelector('#ask').open`))
	run(js(`document.querySelector('#askText').value='Green'; document.querySelector('#askForm').requestSubmit()`))
	run(poll(`!document.querySelector('#ask').open && document.querySelector('#messages').textContent.includes('Thanks for the custom answer.')`))

	// Disconnect the browser from the server. The UI must show it and prevent sending.
	run(js(`window.dispatchEvent(new Event('offline'))`))
	check(`document.querySelector('#sendBtn').disabled`)
}
