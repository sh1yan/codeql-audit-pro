package assistant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HistoryEntry 表示历史记录中的一次 CodeQL 运行。
type HistoryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Config     Config    `json:"config"`
	Action     string    `json:"action,omitempty"` // create / analyze
	Status     string    `json:"status"`
	OutputFile string    `json:"output_file"`
	DurationMs int64     `json:"duration_ms"`
}

// History 存储运行历史，线程安全，并自动持久化。
type History struct {
	mu       sync.Mutex
	Entries  []HistoryEntry `json:"entries"`
	filePath string
}

// NewHistory 从默认位置创建或加载历史记录。
func NewHistory() *History {
	EnsureDataDir()
	h := &History{
		Entries:  []HistoryEntry{},
		filePath: filepath.Join(DataDir(), "history.json"),
	}
	h.load()
	return h
}

// AddEntry 在历史记录开头插入一条，最多保留 100 条。
func (h *History) AddEntry(config Config, action, status string, duration time.Duration, outputFile string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := HistoryEntry{
		Timestamp:  time.Now(),
		Config:     config,
		Action:     action,
		Status:     status,
		DurationMs: duration.Milliseconds(),
		OutputFile: outputFile,
	}
	h.Entries = append([]HistoryEntry{entry}, h.Entries...)
	if len(h.Entries) > 100 {
		h.Entries = h.Entries[:100]
	}
	h.saveLocked()
}

// Snapshot 返回历史记录副本。
func (h *History) Snapshot() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]HistoryEntry, len(h.Entries))
	copy(out, h.Entries)
	return out
}

// Clear 清空所有历史记录。
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Entries = nil
	h.saveLocked()
}

func (h *History) saveLocked() {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(h.filePath), 0o755)
	_ = os.WriteFile(h.filePath, data, 0o644)
}

func (h *History) load() {
	data, err := os.ReadFile(h.filePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, h)
	if h.Entries == nil {
		h.Entries = []HistoryEntry{}
	}
}
