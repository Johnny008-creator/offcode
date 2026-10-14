package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/Johnny008-creator/offcode/internal/setup"
	"github.com/Johnny008-creator/offcode/internal/tui"
	"github.com/Johnny008-creator/offcode/internal/updater"
)

func runUpdate(args []string) {
	fs := flag.NewFlagSet("offcode update", flag.ExitOnError)
	check := fs.Bool("check", false, "only report a new release")
	fs.Parse(args)
	if err := updateOffcode(*check, false); err != nil {
		failExtra(err)
	}
}
func startupUpdate() {
	if os.Getenv("OFFCODE_SKIP_UPDATE") == "1" || !updater.Stable(tui.AppVersion) {
		return
	}
	settings, err := setup.Load()
	if err != nil || settings.Updates == "off" {
		return
	}
	if err = updateOffcode(settings.Updates == "notify", true); err != nil {
		fmt.Fprintln(os.Stderr, "offcode: update skipped; continuing with installed version:", err)
	}
}
func updateOffcode(checkOnly, startup bool) error {
	if !updater.Stable(tui.AppVersion) {
		return fmt.Errorf("development build: install a tagged release to enable updates")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	release, err := updater.New().Check(ctx)
	if err != nil {
		return err
	}
	if !updater.Newer(release.Tag, tui.AppVersion) {
		if !startup {
			fmt.Fprintln(os.Stderr, "offcode: already up to date (v"+tui.AppVersion+")")
		}
		return nil
	}
	fmt.Fprintf(os.Stderr, "offcode: new version %s available (installed v%s)\n", release.Tag, tui.AppVersion)
	if checkOnly {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "offcode: downloading and verifying update...")
	if err = updater.Install(ctx, release, exe); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "offcode: updated to", release.Tag)
	if startup {
		cmd := exec.Command(exe, os.Args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Suppress exactly this launch's check without disabling checks in tools
		// started by the new process (main consumes and removes the variable).
		cmd.Env = append(os.Environ(), "OFFCODE_UPDATE_RESTART="+strings.TrimPrefix(release.Tag, "v"))
		if err = cmd.Run(); err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				os.Exit(e.ExitCode())
			}
			return fmt.Errorf("updated, but relaunch failed: %w; start Offcode again", err)
		}
		os.Exit(0)
	}
	return nil
}
