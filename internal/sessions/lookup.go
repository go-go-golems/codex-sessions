package sessions

import "errors"

var ErrSessionNotFound = errors.New("session not found")

// FindSessionByID scans the sessions root and returns the first session whose meta id matches.
func FindSessionByID(root string, sessionID string) (SessionMeta, error) {
	paths, err := DiscoverRolloutFiles(root)
	if err != nil {
		return SessionMeta{}, err
	}
	for _, p := range paths {
		meta, err := ReadSessionMeta(p)
		if err != nil {
			continue
		}
		if meta.ID == sessionID {
			return meta, nil
		}
	}
	return SessionMeta{}, ErrSessionNotFound
}
