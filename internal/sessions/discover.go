package sessions

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type DiscoverOptions struct {
	// IncludeFilenameCopies includes rollout-*.jsonl files whose filename contains "-copy".
	// These are typically artifacts and are excluded by default.
	IncludeFilenameCopies bool
	// IncludeReflectionCopies includes sessions detected as reflection copies (content-based).
	// These are excluded by default.
	IncludeReflectionCopies bool
	// ReflectionCopyPrefix is the prefix used to detect reflection copies.
	// If empty, reflection copy detection is skipped.
	ReflectionCopyPrefix string
}

// DiscoverRolloutFiles returns sorted rollout-*.jsonl paths under the given root.
func DiscoverRolloutFiles(root string) ([]string, error) {
	return DiscoverRolloutFilesWithOptions(root, DiscoverOptions{
		IncludeFilenameCopies:   false,
		IncludeReflectionCopies: false,
		ReflectionCopyPrefix:    DefaultSelfReflectionPrefix,
	})
}

// DiscoverRolloutFilesWithOptions returns sorted rollout-*.jsonl paths under the given root,
// applying optional filtering for reflection copies.
func DiscoverRolloutFilesWithOptions(root string, opts DiscoverOptions) ([]string, error) {
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
		if !opts.IncludeFilenameCopies && strings.Contains(name, "-copy") {
			return nil
		}
		if !opts.IncludeReflectionCopies && strings.TrimSpace(opts.ReflectionCopyPrefix) != "" {
			isCopy, err := IsReflectionCopy(path, opts.ReflectionCopyPrefix)
			if err == nil && isCopy {
				return nil
			}
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
