package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var apiErrors = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "mojira_api_errors",
	Help: "Number of errors coming from the APIs",
}, []string{"source"})

func NewApiError(source string, err error) error {
	apiErrors.WithLabelValues(source).Inc()
	return fmt.Errorf("API error %s: %s", source, err)
}

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
	if at == 0 {
		s, _ = strings.CutPrefix(s, "@")
		return "@" + SafeName(s)
	}
	if at >= 0 {
		return s[:at]
	}
	return s
}
