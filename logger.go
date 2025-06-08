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
	"strings"
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
	buffer [][3]string // [timestamp, level, message]
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
	level, msg := l.parseLog(p)
	l.mu.Lock()
	l.buffer = append(l.buffer, [3]string{ts, level, msg})
	l.mu.Unlock()
	return len(p), nil
}

func (l *LokiLogger) parseLog(p []byte) (string, string) {
	parts := bytes.SplitN(p, []byte(" "), 3)
	rest := bytes.TrimRight(p, "\n")
	if len(parts) >= 3 {
		rest = parts[2]
	}
	msg := string(rest)
	level := "info"
	if strings.HasPrefix(msg, "[WARNING] ") {
		level = "warning"
		msg, _ = strings.CutPrefix(msg, "[WARNING] ")
	} else if strings.HasPrefix(msg, "[ERROR] ") {
		level = "error"
		msg, _ = strings.CutPrefix(msg, "[ERROR] ")
	}
	return level, msg
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

	streams := make([]map[string]any, len(l.buffer))
	for i, pair := range l.buffer {
		streams[i] = map[string]any{
			"stream": map[string]string{
				"service_name": "mojira",
				"level":        pair[1],
			},
			"values": [][]string{{pair[0], pair[2]}},
		}
	}
	l.buffer = nil
	l.mu.Unlock()

	body := map[string]any{
		"streams": streams,
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
