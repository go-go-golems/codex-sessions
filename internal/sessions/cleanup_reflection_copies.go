package sessions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CleanupReflectionCopiesOptions struct {
	DryRun bool
	Limit  int
	Mode   string // delete|trash

	Project string
	Since   *time.Time
	Until   *time.Time

	Now func() time.Time
}

type CleanupReflectionCopyResult struct {
	Path      string
	DestPath  string
	SizeBytes int64
	SessionID string
	Project   string
	Status    string // would_delete|deleted|would_trash|trashed|error
	Error     string
}

func CleanupReflectionCopies(root string, prefix string, opts CleanupReflectionCopiesOptions) ([]CleanupReflectionCopyResult, error) {
	if strings.TrimSpace(opts.Mode) == "" {
		opts.Mode = "delete"
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

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

		var sizeBytes int64
		if st, err := os.Stat(p); err == nil {
			sizeBytes = st.Size()
		}

		meta, metaErr := ReadSessionMeta(p)
		r := CleanupReflectionCopyResult{
			Path:      filepath.Clean(p),
			SizeBytes: sizeBytes,
			Status:    "would_delete",
		}
		if metaErr == nil {
			r.SessionID = meta.ID
			r.Project = meta.ProjectName()
		} else if opts.Project != "" || opts.Since != nil || opts.Until != nil {
			// If filters are set, we need meta to apply them safely.
			r.Status = "error"
			r.Error = metaErr.Error()
			results = append(results, r)
			continue
		}

		if opts.Project != "" && r.Project != opts.Project {
			continue
		}
		if metaErr == nil && opts.Since != nil && meta.Timestamp.Before(*opts.Since) {
			continue
		}
		if metaErr == nil && opts.Until != nil && meta.Timestamp.After(*opts.Until) {
			continue
		}

		if !opts.DryRun {
			switch opts.Mode {
			case "delete":
				if err := os.Remove(p); err != nil {
					r.Status = "error"
					r.Error = err.Error()
				} else {
					r.Status = "deleted"
				}
			case "trash":
				r.Status = "would_trash"
				dest, err := moveToTrash(p, root, opts.Now().UTC())
				if err != nil {
					r.Status = "error"
					r.Error = err.Error()
				} else {
					r.Status = "trashed"
					r.DestPath = filepath.Clean(dest)
				}
			default:
				r.Status = "error"
				r.Error = fmt.Sprintf("invalid mode: %q (expected delete|trash)", opts.Mode)
			}
		}
		if opts.DryRun && opts.Mode == "trash" {
			r.Status = "would_trash"
		}

		results = append(results, r)
		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}

func moveToTrash(srcPath string, sessionsRoot string, now time.Time) (string, error) {
	trashDir := filepath.Join(
		sessionsRoot,
		"trash",
		"reflection-copies",
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
	)
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return "", err
	}

	base := filepath.Base(srcPath)
	destPath := filepath.Join(trashDir, base)
	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		destPath = filepath.Join(trashDir, fmt.Sprintf("%s-%d%s", stem, now.UnixNano(), ext))
	}

	if err := os.Rename(srcPath, destPath); err == nil {
		return destPath, nil
	}
	// Fall back to copy+remove (e.g., if rename fails due to cross-device issues).
	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(destPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(destPath)
		return "", closeErr
	}
	if err := os.Remove(srcPath); err != nil {
		return "", err
	}
	return destPath, nil
}
