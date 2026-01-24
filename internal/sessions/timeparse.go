package sessions

import "time"

// ParseDateOrDateTime parses either YYYY-MM-DD (assumed UTC midnight) or an RFC3339-like timestamp.
func ParseDateOrDateTime(value string) (time.Time, error) {
	if len(value) == 10 {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return time.Time{}, err
		}
		return parsed.UTC(), nil
	}
	return parseTimestamp(value)
}
