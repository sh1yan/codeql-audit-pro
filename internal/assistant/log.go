package assistant

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// maxLogFileSize 限制单个日志文件的体积，超出后停止写入。
	maxLogFileSize = 64 << 20
	// logRetention 之后的历史日志会在新建日志时被清理。
	logRetention = 3 * 24 * time.Hour
)

// RunLogger 把一次运行的完整输出写入日志文件。
// 界面上的控制台只保留最近若干行，完整内容始终可以从日志文件查看。
type RunLogger struct {
	mu     sync.Mutex
	file   *os.File
	path   string
	bytes  int64
	closed bool
}

// StartRunLog 新建一个运行日志文件，并清理过期日志。
func StartRunLog() *RunLogger {
	logger := &RunLogger{}
	EnsureDataDir()
	dir := LogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return logger
	}
	cleanupOldLogs(dir)

	name := time.Now().Format("20060102-150405.000") + ".log"
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return logger
	}
	logger.file = file
	logger.path = path
	return logger
}

func cleanupOldLogs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-logRetention)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// WriteLine 写入一行完整日志。达到体积上限后静默停止写入。
func (l *RunLogger) WriteLine(line string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.file == nil || l.bytes >= maxLogFileSize {
		return
	}
	n, err := fmt.Fprintln(l.file, line)
	if err != nil {
		return
	}
	l.bytes += int64(n)
}

// Close 关闭日志文件。
func (l *RunLogger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	if l.file != nil {
		_ = l.file.Close()
	}
}

// Path 返回日志文件路径（创建失败时为空串）。
func (l *RunLogger) Path() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.path
}
