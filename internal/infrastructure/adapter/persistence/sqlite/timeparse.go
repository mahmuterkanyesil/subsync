package sqlite

import (
	"database/sql"
	"time"
)

// parseTime parses common datetime formats returned by SQLite into time.Time.
// If the value is not valid, it returns the zero time.
func parseTime(ns sql.NullString) time.Time {
    if !ns.Valid {
        return time.Time{}
    }
    layouts := []string{
        time.RFC3339,
        time.RFC3339Nano,
        "2006-01-02 15:04:05",
        "2006-01-02 15:04:05.999999",
        "2006-01-02 15:04:05.999999999",
        "2006-01-02 15:04",
        "2006-01-02T15:04:05Z",
        "2006-01-02T15:04:05.999999Z",
        "2006-01-02T15:04:05.999999999Z",
    }
    for _, l := range layouts {
        if t, err := time.Parse(l, ns.String); err == nil {
            return t
        }
    }
    // fallback: attempt ParseInLocation with common layout
    if t, err := time.ParseInLocation("2006-01-02 15:04:05", ns.String, time.Local); err == nil {
        return t
    }
    return time.Time{}
}

func parseTimePtr(ns sql.NullString) *time.Time {
    t := parseTime(ns)
    if t.IsZero() {
        return nil
    }
    return &t
}
