package buildtags

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Some minimal regexes to detect build lines and package lines.
// Expand/refine as needed.
var (
	reGoBuild   = regexp.MustCompile(`^//\s*go:build\s+(.+)$`)
	rePlusBuild = regexp.MustCompile(`^//\s*\+build\s+(.+)$`)
	rePackage   = regexp.MustCompile(`^\s*package\s+[\p{L}_]\w*`)
)

// WalkDir finds files with specified extensions.
// This is the naive approach—tailor as you like.
func WalkDir(root string, exts []string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(path, "."+ext) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	return files, err
}

// parseBuildLine attempts to extract tags from a single line.
// For //go:build lines, we just split on whitespace.
// For // +build lines, we split on whitespace, then split each chunk by commas.
func parseBuildLine(line string) []string {
	line = strings.TrimSpace(line)

	// Handle //go:build ...
	if m := reGoBuild.FindStringSubmatch(line); m != nil {
		// naive: just split on spaces
		return strings.Fields(m[1])
	}

	// Handle // +build ...
	if m := rePlusBuild.FindStringSubmatch(line); m != nil {
		// in "+build" lines, multiple tags can appear separated by spaces or commas
		var tags []string
		for _, chunk := range strings.Fields(m[1]) {
			tags = append(tags, strings.Split(chunk, ",")...)
		}
		return tags
	}

	return nil
}

// extractBuildTagsFromFile reads only the first ~512 bytes, splits them into lines,
// and scans for build tags (//go:build or // +build). Stops upon seeing a package line.
func extractBuildTagsFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read a limited amount—adjust to taste.
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	var tags []string
	scanner := bufio.NewScanner(bytes.NewReader(buf[:n]))
	for scanner.Scan() {
		line := scanner.Text()

		// Stop if we hit the package line.
		if rePackage.MatchString(line) {
			break
		}

		found := parseBuildLine(line)
		tags = append(tags, found...)
	}
	return tags, scanner.Err()
}

// extractBuildTagsFromDir aggregates all build tags found in *.go files under rootDir.
func ExtractBuildTagsFromDir(rootDir string) ([]string, error) {
	files, err := WalkDir(rootDir, []string{"go"})
	if err != nil {
		return nil, err
	}
	unique := make(map[string]bool)
	for _, file := range files {
		tags, err := extractBuildTagsFromFile(file)
		if err != nil {
			// handle error
			continue
		}
		for _, t := range tags {
			unique[t] = true
		}
	}
	var result []string
	for t := range unique {
		result = append(result, t)
	}
	return result, nil
}
