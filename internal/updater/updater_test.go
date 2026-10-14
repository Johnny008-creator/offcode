package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"v0.2.10", "0.2.9", true}, {"v1.0.0", "0.99.99", true}, {"v0.2.1", "0.2.1", false},
		{"v0.2.0", "0.2.1", false}, {"v1.0.0-rc1", "0.2.1", false}, {"v1.0.0", "0.2.2-dev", false},
		{"v01.2.3", "0.2.1", false}, {"garbage", "0.2.1", false},
	} {
		if got := Newer(tc.a, tc.b); got != tc.want {
			t.Fatalf("%s > %s: %v", tc.a, tc.b, got)
		}
	}
}
func TestReleaseCheck(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
		ok   bool
	}{
		{`{"tag_name":"v0.3.0"}`, 200, true},
		{`{"tag_name":"v0.3.0","prerelease":true}`, 200, false},
		{`{"tag_name":"v0.3.0","draft":true}`, 200, false},
		{`{`, 200, false}, {`rate limited`, 403, false},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); io.WriteString(w, tc.body) }))
		_, err := (Client{HTTP: srv.Client(), Latest: srv.URL}).Check(context.Background())
		srv.Close()
		if (err == nil) != tc.ok {
			t.Fatal(tc, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Check(ctx); err == nil {
		t.Fatal("offline/cancellation not returned")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestVerifiedInstall(t *testing.T) {
	name, err := assetName()
	if err != nil {
		t.Skip(err)
	}
	for _, scenario := range []string{"valid", "bad checksum", "truncated", "missing checksum", "download fails"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			exe := filepath.Join(dir, "offcode")
			os.WriteFile(exe, []byte("old"), 0755)
			payload := "new version"
			sum := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(payload)), name)
			switch scenario {
			case "bad checksum":
				sum = strings.Repeat("0", 64) + "  " + name
			case "missing checksum":
				sum = ""
			}
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
				body := payload
				code := 200
				if strings.HasSuffix(r.URL.Path, "SHA256SUMS") {
					body = sum
				} else {
					if scenario == "truncated" {
						body = "new"
					}
					if scenario == "download fails" {
						code = 503
					}
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			release := Release{Tag: "v0.3.0", Assets: []Asset{
				{Name: name, URL: assetPrefix + "v0.3.0/" + name, Size: int64(len(payload))},
				{Name: "SHA256SUMS", URL: assetPrefix + "v0.3.0/SHA256SUMS"},
			}}
			err := Install(context.Background(), release, exe)
			want := "old"
			if scenario == "valid" {
				want = payload
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("bad update accepted")
			}
			got, _ := os.ReadFile(exe)
			if string(got) != want {
				t.Fatalf("executable=%q", got)
			}
			unlock, err := acquireLock(exe + ".update-lock")
			if err != nil {
				t.Fatal("lock not released", err)
			}
			unlock()
		})
	}
}
func TestRejectForeignAsset(t *testing.T) {
	r := Release{Tag: "v0.3.0", Assets: []Asset{{Name: "binary", URL: "https://example.com/binary"}}}
	if _, err := r.asset("binary"); err == nil {
		t.Fatal("foreign download allowed")
	}
}
func TestReplaceRollback(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "offcode")
	os.WriteFile(exe, []byte("old"), 0755)
	if err := replace(exe, exe+".nonexistent"); err == nil {
		t.Fatal("expected failure")
	}
	got, err := os.ReadFile(exe)
	if err != nil || string(got) != "old" {
		t.Fatal("rollback failed", err)
	}
}
func TestReplaceRunningExecutable(t *testing.T) {
	if os.Getenv("OFFCODE_UPDATER_TEST_CHILD") == "1" {
		fmt.Println("ready")
		io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "running.exe")
	stage := filepath.Join(dir, "new.exe")
	for _, p := range []string{target, stage} {
		if err = os.WriteFile(p, bytes, 0755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(target, "-test.run=^TestReplaceRunningExecutable$")
	cmd.Env = append(os.Environ(), "OFFCODE_UPDATER_TEST_CHILD=1")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); cmd.Wait() }()
	ready := make(chan error, 1)
	go func() { buf := make([]byte, 5); _, err := io.ReadFull(stdout, buf); ready <- err }()
	select {
	case err = <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("child timed out")
	}
	if err = replace(target, stage); err != nil {
		t.Fatal("cannot update a running executable:", err)
	}
}

func TestConcurrentUpdateLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	release, err := acquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := acquireLock(path); err == nil {
		second()
		t.Error("concurrent updater acquired lock")
	}
	release()
	next, err := acquireLock(path)
	if err != nil {
		t.Fatal("lock not released", err)
	}
	next()
}
