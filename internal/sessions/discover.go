package sessions

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverRolloutFiles returns sorted rollout-*.jsonl paths under the given root.
//
// This mirrors the Python behavior: it scans recursively and excludes files that
// look like reflection copies.
func DiscoverRolloutFiles(root string) ([]string, error) {
	var paths []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			return nil
		}
		// The Python tool excludes "-copy" artifacts; keep the same heuristic.
		if strings.Contains(name, "-copy") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(paths)
	return paths, nil
}
