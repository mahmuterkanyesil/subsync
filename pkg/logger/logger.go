package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

var current level = LevelInfo

// in-memory ring buffer for recent logs
type LogEntry struct {
    Time    time.Time `json:"time"`
    Level   string    `json:"level"`
    Message string    `json:"message"`
}

var (
    bufMu   sync.Mutex
    buf     []LogEntry
    bufSize = 2000
    forwardURL string
)

func Init() {
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
    // optional forwarding endpoint to central API (e.g. http://api:8080/internal/logs)
    forwardURL = os.Getenv("LOG_FORWARD_URL")
}

func enabled(l level) bool {
    return l >= current
}

func Debug(format string, v ...interface{}) {
    if !enabled(LevelDebug) {
        return
    }
    msg := fmt.Sprintf(format, v...)
    log.Output(2, "DEBUG: "+msg)
    pushEntry("debug", msg)
}

func Info(format string, v ...interface{}) {
    if !enabled(LevelInfo) {
        return
    }
    msg := fmt.Sprintf(format, v...)
    log.Output(2, "INFO: "+msg)
    pushEntry("info", msg)
}

func Warn(format string, v ...interface{}) {
    if !enabled(LevelWarn) {
        return
    }
    msg := fmt.Sprintf(format, v...)
    log.Output(2, "WARN: "+msg)
    pushEntry("warn", msg)
}

func Error(format string, v ...interface{}) {
    if !enabled(LevelError) {
        return
    }
    msg := fmt.Sprintf(format, v...)
    log.Output(2, "ERROR: "+msg)
    pushEntry("error", msg)
}

// SetLevel allows programmatic override of the level
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
    e := LogEntry{Time: time.Now().UTC(), Level: levelStr, Message: msg}
    if len(buf) < bufSize {
        buf = append(buf, e)
        // forward asynchronously if configured
        go forwardAsync(levelStr, msg, e.Time)
        return
    }
    // ring: drop oldest
    copy(buf[0:], buf[1:])
    buf[len(buf)-1] = e
    go forwardAsync(levelStr, msg, e.Time)
}

// ReceiveRemote allows other services (API) to push remote-forwarded logs into this process buffer
func ReceiveRemote(t time.Time, levelStr, msg string) {
    bufMu.Lock()
    defer bufMu.Unlock()
    if buf == nil {
        buf = make([]LogEntry, 0, bufSize)
    }
    e := LogEntry{Time: t.UTC(), Level: levelStr, Message: msg}
    if len(buf) < bufSize {
        buf = append(buf, e)
        return
    }
    copy(buf[0:], buf[1:])
    buf[len(buf)-1] = e
}

func forwardAsync(levelStr, msg string, t time.Time) {
    if forwardURL == "" {
        return
    }
    // fire-and-forget
    go func() {
        type payload struct {
            Time    string `json:"time"`
            Level   string `json:"level"`
            Message string `json:"message"`
        }
        p := payload{Time: t.UTC().Format(time.RFC3339), Level: levelStr, Message: msg}
        b, _ := jsonMarshal(p)
        // best-effort POST
        _, _ = httpPost(forwardURL, "application/json", b)
    }()
}

// lightweight wrappers to avoid importing net/http and encoding/json in hot path
func jsonMarshal(v interface{}) ([]byte, error) {
    // import locally to keep top-level imports minimal
    return func() ([]byte, error) {
        type enc func(interface{}) ([]byte, error)
        return json.Marshal(v)
    }()
}

func httpPost(url, contentType string, body []byte) ([]byte, error) {
    return func() ([]byte, error) {
        client := &http.Client{Timeout: 2 * time.Second}
        resp, err := client.Post(url, contentType, bytes.NewReader(body))
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
        return ioReadAll(resp.Body)
    }()
}

func ioReadAll(r io.Reader) ([]byte, error) {
    return ioutil.ReadAll(r)
}

// GetRecent returns up to n recent log entries (newest last)
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
