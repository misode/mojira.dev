package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

func NewFileLogger(filename string) io.Writer {
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}
	file, err := os.OpenFile(filepath.Join("logs", filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	return file
}

type LokiLogger struct {
	mu     sync.Mutex
	buffer [][2]string // [timestamp, message]
	ticker *time.Ticker
	client *http.Client
	token  string
}

func NewLokiLogger(interval time.Duration) *LokiLogger {
	l := &LokiLogger{
		client: &http.Client{},
		token:  os.Getenv("LOKI_TOKEN"),
	}
	if l.token != "" {
		l.ticker = time.NewTicker(interval)
		go l.run()
	}
	return l
}

func (l *LokiLogger) Write(p []byte) (int, error) {
	if l.token == "" {
		return len(p), nil
	}
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	msg := l.formatMessage(p)
	l.mu.Lock()
	l.buffer = append(l.buffer, [2]string{ts, msg})
	l.mu.Unlock()
	return len(p), nil
}

func (l *LokiLogger) formatMessage(p []byte) string {
	parts := bytes.SplitN(p, []byte(" "), 3)
	if len(parts) < 3 {
		return string(bytes.TrimRight(p, "\n"))
	}
	return string(bytes.TrimRight(parts[2], "\n"))
}

func (l *LokiLogger) run() {
	for range l.ticker.C {
		l.flush()
	}
}

func (l *LokiLogger) flush() {
	l.mu.Lock()
	if len(l.buffer) == 0 {
		l.mu.Unlock()
		return
	}

	values := make([][]string, len(l.buffer))
	for i, pair := range l.buffer {
		values[i] = []string{pair[0], pair[1]}
	}
	l.buffer = nil
	l.mu.Unlock()

	body := map[string]any{
		"streams": []map[string]any{
			{
				"stream": map[string]string{
					"service_name": "mojira",
					"level":        "info",
				},
				"values": values,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)

	url := "https://logs-prod-012.grafana.net/loki/api/v1/push"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("[loki] Failed to build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer 1235352:"+l.token)

	resp, err := l.client.Do(req)
	if err != nil {
		log.Printf("[loki] Failed to send logs: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[loki] Error: %s", body)
	}
}
