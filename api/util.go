package api

import (
	"strings"
	"time"
)

func ParseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return &t, nil
	}
	t, err = time.Parse("2006-01-02T15:04:05.000-0700", s)
	if err == nil {
		return &t, nil
	}
	t, err = time.Parse("2006-01-02T15:04:05-0700", s)
	if err == nil {
		return &t, nil
	}
	return nil, err
}

func SafeName(s string) string {
	at := strings.IndexByte(s, '@')
	if at >= 0 {
		return s[:at]
	}
	return s
}
