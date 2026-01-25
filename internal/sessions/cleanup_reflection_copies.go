package sessions

import (
	"os"
	"path/filepath"
)

type CleanupReflectionCopiesOptions struct {
	DryRun bool
	Limit  int
}

type CleanupReflectionCopyResult struct {
	Path      string
	SessionID string
	Project   string
	Status    string // would_delete|deleted|error
	Error     string
}

func CleanupReflectionCopies(root string, prefix string, opts CleanupReflectionCopiesOptions) ([]CleanupReflectionCopyResult, error) {
	paths, err := DiscoverRolloutFilesWithOptions(root, DiscoverOptions{
		IncludeFilenameCopies:   true,
		IncludeReflectionCopies: true,
		ReflectionCopyPrefix:    "",
	})
	if err != nil {
		return nil, err
	}

	results := make([]CleanupReflectionCopyResult, 0)
	for _, p := range paths {
		isCopy, err := IsReflectionCopy(p, prefix)
		if err != nil {
			results = append(results, CleanupReflectionCopyResult{
				Path:   filepath.Clean(p),
				Status: "error",
				Error:  err.Error(),
			})
			continue
		}
		if !isCopy {
			continue
		}

		meta, metaErr := ReadSessionMeta(p)
		r := CleanupReflectionCopyResult{
			Path:   filepath.Clean(p),
			Status: "would_delete",
		}
		if metaErr == nil {
			r.SessionID = meta.ID
			r.Project = meta.ProjectName()
		}

		if !opts.DryRun {
			if err := os.Remove(p); err != nil {
				r.Status = "error"
				r.Error = err.Error()
			} else {
				r.Status = "deleted"
			}
		}

		results = append(results, r)
		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}
