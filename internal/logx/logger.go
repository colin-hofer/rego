package logx

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiCyan   = "\x1b[36m"
	ansiBlue   = "\x1b[34m"
)

type Logger struct {
	out       io.Writer
	minLevel  Level
	color     bool
	component string
	mu        *sync.Mutex
}

func New(minLevel Level) *Logger {
	return &Logger{
		out:       os.Stdout,
		minLevel:  minLevel,
		color:     shouldUseColor(os.Stdout),
		component: "app",
		mu:        &sync.Mutex{},
	}
}

func ParseLevel(raw string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return DebugLevel, true
	case "info":
		return InfoLevel, true
	case "warn", "warning":
		return WarnLevel, true
	case "error":
		return ErrorLevel, true
	default:
		return InfoLevel, false
	}
}

func (l *Logger) WithComponent(component string) *Logger {
	clone := *l
	clone.component = strings.TrimSpace(component)
	if clone.component == "" {
		clone.component = "app"
	}
	return &clone
}

func (l *Logger) Debug(msg string, kv ...any) {
	l.log(DebugLevel, msg, kv...)
}

func (l *Logger) Info(msg string, kv ...any) {
	l.log(InfoLevel, msg, kv...)
}

func (l *Logger) Warn(msg string, kv ...any) {
	l.log(WarnLevel, msg, kv...)
}

func (l *Logger) Error(msg string, kv ...any) {
	l.log(ErrorLevel, msg, kv...)
}

func (l *Logger) log(level Level, msg string, kv ...any) {
	if level < l.minLevel {
		return
	}

	parts := make([]string, 0, 5)
	parts = append(parts, l.formatTime(time.Now()))
	parts = append(parts, l.formatLevel(level))
	parts = append(parts, l.formatComponent())
	parts = append(parts, msg)

	fields := formatKV(kv)
	if fields != "" {
		parts = append(parts, fields)
	}

	line := strings.Join(parts, " ")

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintln(l.out, line)
}

func (l *Logger) formatTime(now time.Time) string {
	ts := now.Format("15:04:05.000")
	if !l.color {
		return ts
	}
	return ansiDim + ts + ansiReset
}

func (l *Logger) formatLevel(level Level) string {
	label := levelLabel(level)
	if !l.color {
		return label
	}

	color := ansiGreen
	switch level {
	case DebugLevel:
		color = ansiCyan
	case WarnLevel:
		color = ansiYellow
	case ErrorLevel:
		color = ansiRed
	}

	return color + label + ansiReset
}

func (l *Logger) formatComponent() string {
	if !l.color {
		return "[" + l.component + "]"
	}
	return ansiBlue + "[" + l.component + "]" + ansiReset
}

func levelLabel(level Level) string {
	switch level {
	case DebugLevel:
		return "DBG"
	case InfoLevel:
		return "INF"
	case WarnLevel:
		return "WRN"
	case ErrorLevel:
		return "ERR"
	default:
		return "UNK"
	}
}

func formatKV(kv []any) string {
	if len(kv) == 0 {
		return ""
	}

	if len(kv)%2 != 0 {
		kv = append(kv, "<missing>")
	}

	parts := make([]string, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key := strings.TrimSpace(fmt.Sprint(kv[i]))
		if key == "" {
			key = "field"
		}
		parts = append(parts, key+"="+fmt.Sprint(kv[i+1]))
	}

	return strings.Join(parts, " ")
}

func shouldUseColor(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}

	file, ok := out.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
