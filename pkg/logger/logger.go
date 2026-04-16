package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type level int

const (
	LevelDebug level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	current     level  = LevelInfo
	serviceName string = "unknown"
)

// LogEntry is stored in the in-memory ring buffer and forwarded to the API.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Service string    `json:"service"`
	Message string    `json:"message"`
}

var (
	bufMu      sync.Mutex
	buf        []LogEntry
	bufSize    = 2000
	forwardURL string
)

func Init() {
	// Log level
	lv := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch lv {
	case "debug":
		current = LevelDebug
	case "info":
		current = LevelInfo
	case "warn":
		current = LevelWarn
	case "error":
		current = LevelError
	default:
		current = LevelInfo
	}

	// Service name: LOG_SERVICE env var takes priority, else binary name
	if svc := os.Getenv("LOG_SERVICE"); svc != "" {
		serviceName = svc
	} else if len(os.Args) > 0 {
		name := filepath.Base(os.Args[0])
		name = strings.TrimSuffix(name, filepath.Ext(name))
		serviceName = name
	}

	// Optional forwarding endpoint to central API
	forwardURL = os.Getenv("LOG_FORWARD_URL")
}

func enabled(l level) bool {
	return l >= current
}

func writeLine(lvl, msg string) {
	now := time.Now().Format("15:04:05")
	svc := fmt.Sprintf("%-8s", serviceName)
	l := fmt.Sprintf("%-5s", strings.ToUpper(lvl))
	fmt.Fprintf(os.Stderr, "%s [%s] %s  %s\n", now, svc, l, msg)
}

func Debug(format string, v ...interface{}) {
	if !enabled(LevelDebug) {
		return
	}
	msg := fmt.Sprintf(format, v...)
	writeLine("debug", msg)
	pushEntry("debug", msg)
}

func Info(format string, v ...interface{}) {
	if !enabled(LevelInfo) {
		return
	}
	msg := fmt.Sprintf(format, v...)
	writeLine("info", msg)
	pushEntry("info", msg)
}

func Warn(format string, v ...interface{}) {
	if !enabled(LevelWarn) {
		return
	}
	msg := fmt.Sprintf(format, v...)
	writeLine("warn", msg)
	pushEntry("warn", msg)
}

func Error(format string, v ...interface{}) {
	if !enabled(LevelError) {
		return
	}
	msg := fmt.Sprintf(format, v...)
	writeLine("error", msg)
	pushEntry("error", msg)
}

// SetLevel allows programmatic override of the level.
func SetLevel(l string) {
	os.Setenv("LOG_LEVEL", strings.ToLower(l))
	Init()
}

func pushEntry(levelStr, msg string) {
	bufMu.Lock()
	defer bufMu.Unlock()
	if buf == nil {
		buf = make([]LogEntry, 0, bufSize)
	}
	e := LogEntry{Time: time.Now().UTC(), Level: levelStr, Service: serviceName, Message: msg}
	if len(buf) < bufSize {
		buf = append(buf, e)
		go forwardAsync(levelStr, serviceName, msg, e.Time)
		return
	}
	// ring: drop oldest
	copy(buf[0:], buf[1:])
	buf[len(buf)-1] = e
	go forwardAsync(levelStr, serviceName, msg, e.Time)
}

// ReceiveRemote allows the API to push remote-forwarded logs into this process buffer.
func ReceiveRemote(t time.Time, service, levelStr, msg string) {
	bufMu.Lock()
	defer bufMu.Unlock()
	if buf == nil {
		buf = make([]LogEntry, 0, bufSize)
	}
	e := LogEntry{Time: t.UTC(), Level: levelStr, Service: service, Message: msg}
	if len(buf) < bufSize {
		buf = append(buf, e)
		return
	}
	copy(buf[0:], buf[1:])
	buf[len(buf)-1] = e
}

func forwardAsync(levelStr, service, msg string, t time.Time) {
	if forwardURL == "" {
		return
	}
	go func() {
		type payload struct {
			Time    string `json:"time"`
			Level   string `json:"level"`
			Service string `json:"service"`
			Message string `json:"message"`
		}
		p := payload{
			Time:    t.UTC().Format(time.RFC3339),
			Level:   levelStr,
			Service: service,
			Message: msg,
		}
		b, _ := jsonMarshal(p)
		_, _ = httpPost(forwardURL, "application/json", b)
	}()
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func httpPost(url, contentType string, body []byte) ([]byte, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(url, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// GetRecent returns up to n recent log entries (newest last).
func GetRecent(n int) []LogEntry {
	bufMu.Lock()
	defer bufMu.Unlock()
	if n <= 0 || n > len(buf) {
		n = len(buf)
	}
	res := make([]LogEntry, n)
	copy(res, buf[len(buf)-n:])
	return res
}
