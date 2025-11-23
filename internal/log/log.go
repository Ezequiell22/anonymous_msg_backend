package log

import (
    "encoding/json"
    "os"
    "strings"
    "time"
)

type Level int

const (
    Debug Level = iota
    Info
    Warn
    Error
)

type Logger interface {
    Debug(msg string, fields map[string]any)
    Info(msg string, fields map[string]any)
    Warn(msg string, fields map[string]any)
    Error(msg string, fields map[string]any)
}

type JSONLogger struct {
    level Level
}

func New(levelStr string) *JSONLogger {
    switch strings.ToLower(levelStr) {
    case "debug":
        return &JSONLogger{level: Debug}
    case "warn":
        return &JSONLogger{level: Warn}
    case "error":
        return &JSONLogger{level: Error}
    default:
        return &JSONLogger{level: Info}
    }
}

func (l *JSONLogger) log(lv Level, msg string, fields map[string]any) {
    if lv < l.level {
        return
    }
    m := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339Nano), "level": levelString(lv), "msg": msg}
    for k, v := range fields {
        m[k] = v
    }
    b, _ := json.Marshal(m)
    os.Stderr.Write(append(b, '\n'))
}

func levelString(lv Level) string {
    switch lv {
    case Debug:
        return "debug"
    case Info:
        return "info"
    case Warn:
        return "warn"
    case Error:
        return "error"
    default:
        return "info"
    }
}

func (l *JSONLogger) Debug(msg string, fields map[string]any) { l.log(Debug, msg, fields) }
func (l *JSONLogger) Info(msg string, fields map[string]any)  { l.log(Info, msg, fields) }
func (l *JSONLogger) Warn(msg string, fields map[string]any)  { l.log(Warn, msg, fields) }
func (l *JSONLogger) Error(msg string, fields map[string]any) { l.log(Error, msg, fields) }
