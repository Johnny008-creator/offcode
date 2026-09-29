package tools

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".offcode":     true,
}

func walkFiles(root string, visit func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		return visit(path)
	})
}

func Grep(pattern, root string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	var matches []string
	const maxMatches = 200

	err = walkFiles(root, func(path string) error {
		if len(matches) >= maxMatches {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if re.MatchString(scanner.Text()) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", path, lineNum, scanner.Text()))
				if len(matches) >= maxMatches {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "no matches", nil
	}
	return strings.Join(matches, "\n"), nil
}

func Glob(pattern, root string) (string, error) {
	var matches []string
	const maxMatches = 500

	err := walkFiles(root, func(path string) error {
		if len(matches) >= maxMatches {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		ok, err := filepath.Match(pattern, rel)
		if err == nil && ok {
			matches = append(matches, rel)
			return nil
		}
		ok, err = filepath.Match(pattern, filepath.Base(path))
		if err == nil && ok {
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "no matches", nil
	}
	return strings.Join(matches, "\n"), nil
}
