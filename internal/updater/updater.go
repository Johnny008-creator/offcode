// Package updater downloads verified release binaries and replaces only the
// executable. User configuration, Ollama and model weights are never touched.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const LatestURL = "https://api.github.com/repos/Johnny008-creator/offcode/releases/latest"
const assetPrefix = "https://github.com/Johnny008-creator/offcode/releases/download/"

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}
type Release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}
type Client struct {
	HTTP   *http.Client
	Latest string
}

func New() Client { return Client{HTTP: &http.Client{Timeout: 5 * time.Second}, Latest: LatestURL} }

func parseVersion(s string) ([3]uint64, bool) {
	var v [3]uint64
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		if !regexp.MustCompile(`^(0|[1-9][0-9]*)$`).MatchString(p) {
			return v, false
		}
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}
func Newer(latest, current string) bool {
	a, ok := parseVersion(latest)
	if !ok {
		return false
	}
	b, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
func Stable(version string) bool { _, ok := parseVersion(version); return ok }
func (c Client) Check(ctx context.Context) (Release, error) {
	var r Release
	req, err := http.NewRequestWithContext(ctx, "GET", c.Latest, nil)
	if err != nil {
		return r, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "offcode-updater")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return r, fmt.Errorf("release check HTTP %d", resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&r)
	if err == nil && (r.Draft || r.Prerelease || !Stable(r.Tag)) {
		err = fmt.Errorf("not a stable release")
	}
	return r, err
}
func (r Release) asset(name string) (Asset, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			// Never execute a binary obtained from an arbitrary URL in metadata.
			if !strings.HasPrefix(a.URL, assetPrefix+r.Tag+"/") {
				return Asset{}, fmt.Errorf("unexpected asset URL")
			}
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s is missing %s", r.Tag, name)
}
func assetName() (string, error) {
	if runtime.GOARCH != "amd64" || (runtime.GOOS != "windows" && runtime.GOOS != "linux") {
		return "", fmt.Errorf("no release binary for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	name := "offcode-" + runtime.GOOS + "-amd64"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name, nil
}
func checksum(data []byte, name string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			raw, err := hex.DecodeString(fields[0])
			if err != nil || len(raw) != sha256.Size {
				return "", fmt.Errorf("invalid SHA256 for %s", name)
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksum missing for %s", name)
}
func download(ctx context.Context, url string, w io.Writer, limit int64) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("insecure asset redirect")
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if n > limit {
		return n, fmt.Errorf("download exceeds size limit")
	}
	return n, err
}
func Install(ctx context.Context, r Release, executable string) error {
	name, err := assetName()
	if err != nil {
		return err
	}
	binary, err := r.asset(name)
	if err != nil {
		return err
	}
	sums, err := r.asset("SHA256SUMS")
	if err != nil {
		return err
	}
	if binary.Size <= 0 || binary.Size > 512<<20 {
		return fmt.Errorf("invalid release binary size")
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	releaseLock, err := acquireLock(executable + ".update-lock")
	if err != nil {
		return fmt.Errorf("cannot lock update (another update may be running): %w", err)
	}
	defer releaseLock()
	var buf strings.Builder
	if _, err = download(ctx, sums.URL, &buf, 1<<20); err != nil {
		return err
	}
	want, err := checksum([]byte(buf.String()), name)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(executable), ".offcode-update-*")
	if err != nil {
		return err
	}
	stage := f.Name()
	defer os.Remove(stage)
	defer f.Close()
	hash := sha256.New()
	n, err := download(ctx, binary.URL, io.MultiWriter(f, hash), binary.Size)
	if err != nil {
		return err
	}
	if n != binary.Size {
		return fmt.Errorf("incomplete binary download")
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return fmt.Errorf("release checksum mismatch; keeping current version")
	}
	info, err := os.Stat(executable)
	if err != nil {
		return err
	}
	if err = f.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return replace(executable, stage)
}

// On Windows a running executable can be renamed but cannot be overwritten.
// Keep it as a rollback copy; remove it on the next successful update.
func replace(executable, stage string) error {
	backup := executable + ".previous"
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("previous binary is in use; close other Offcode windows: %w", err)
	}
	if err := os.Rename(executable, backup); err != nil {
		return err
	}
	if err := os.Rename(stage, executable); err != nil {
		if rollback := os.Rename(backup, executable); rollback != nil {
			return fmt.Errorf("install failed: %v; rollback failed: %v; previous binary: %s", err, rollback, backup)
		}
		return err
	}
	return nil
}
