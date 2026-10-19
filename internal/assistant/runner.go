package assistant

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Status 表示运行器当前状态。使用字符串常量便于直接通过 JSON 传给前端。
type Status string

const (
	StatusIdle      Status = "idle"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
)

// Label 返回状态的中文名称。
func (s Status) Label() string {
	switch s {
	case StatusRunning:
		return "运行中…"
	case StatusCompleted:
		return "已完成"
	case StatusFailed:
		return "失败"
	case StatusStopped:
		return "已停止"
	default:
		return "就绪"
	}
}

// OutputCallback 为 CodeQL 进程输出的每一行调用。
type OutputCallback func(line string, isError bool)

// Runner 管理 CodeQL 进程的整个生命周期。
//
// 同一个 Runner 同时只允许一个任务；任务在后台 goroutine 中执行，
// 因此前端切换到审计工作台、甚至关闭当前页面视图，都不会中断进程。
type Runner struct {
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	cmd           *exec.Cmd
	status        Status
	statusCb      func(Status)
	stopRequested bool
}

// NewRunner 创建一个新的 Runner。
func NewRunner() *Runner {
	return &Runner{status: StatusIdle}
}

// SetStatusCallback 设置状态变更回调。回调在任意 goroutine 中执行，
// 实现方需要自行保证线程安全（这里由 App 的互斥锁负责）。
func (r *Runner) SetStatusCallback(cb func(Status)) {
	r.mu.Lock()
	r.statusCb = cb
	r.mu.Unlock()
}

func (r *Runner) notifyStatus(s Status) {
	r.mu.Lock()
	r.status = s
	cb := r.statusCb
	r.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

// readOutput 带 context 感知地读取输出。
func readOutput(ctx context.Context, reader io.Reader, callback func(string)) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for scanner.Scan() {
			callback(scanner.Text())
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// 进程被取消时不再等待 scanner，避免 Stop 被阻塞。
	}
}

// RunAnalyze 执行 codeql database analyze。
func (r *Runner) RunAnalyze(config Config, outputCb OutputCallback) error {
	return r.run(config, r.BuildAnalyzeArgs(config), "分析", outputCb)
}

// RunDatabaseCreate 执行 codeql database create。
func (r *Runner) RunDatabaseCreate(config Config, outputCb OutputCallback) error {
	return r.run(config, r.BuildDatabaseCreateArgs(config), "数据库创建", outputCb)
}

func (r *Runner) run(config Config, args []string, label string, outputCb OutputCallback) error {
	if outputCb == nil {
		outputCb = func(string, bool) {}
	}

	r.mu.Lock()
	if r.status == StatusRunning {
		r.mu.Unlock()
		return fmt.Errorf("已有任务正在运行，请先停止当前任务")
	}
	if r.stopRequested {
		r.stopRequested = false
		r.mu.Unlock()
		r.notifyStatus(StatusStopped)
		return fmt.Errorf("任务在启动前已被停止")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, config.BinaryPath, args...)
	setProcessGroup(cmd)

	r.ctx = ctx
	r.cancel = cancel
	// 先不暴露 cmd：exec.Cmd.Start() 会写入 cmd.Process，若此时 Stop 读取
	// cmd.Process 会触发数据竞争。等待 Start 成功后再在锁内置入。
	r.cmd = nil
	// 在真正 Start 之前就把状态置为运行中，避免并发调用在启动窗口内重复进入。
	r.status = StatusRunning
	r.mu.Unlock()

	resetRunState := func() {
		r.mu.Lock()
		r.cmd = nil
		r.cancel = nil
		r.stopRequested = false
		r.mu.Unlock()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		resetRunState()
		r.notifyStatus(StatusFailed)
		return fmt.Errorf("创建标准输出管道失败：%w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		resetRunState()
		r.notifyStatus(StatusFailed)
		return fmt.Errorf("创建标准错误管道失败：%w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		resetRunState()
		if ctx.Err() != nil {
			r.notifyStatus(StatusStopped)
			return fmt.Errorf("任务在启动前已被停止")
		}
		outputCb(fmt.Sprintf("启动失败：%v", err), true)
		r.notifyStatus(StatusFailed)
		return fmt.Errorf("启动 CodeQL 失败：%w", err)
	}

	// Start 已返回，cmd.Process 不再被写入；此时再让 Stop 能看到进程。
	r.mu.Lock()
	r.cmd = cmd
	stopAfterStart := r.stopRequested
	r.stopRequested = false
	r.mu.Unlock()
	if stopAfterStart && cmd.Process != nil {
		pid := cmd.Process.Pid
		terminateProcessGroup(pid)
		go func() {
			time.Sleep(500 * time.Millisecond)
			killProcessGroup(pid)
		}()
	}

	r.notifyStatus(StatusRunning)
	outputCb(fmt.Sprintf("$ %s %s", config.BinaryPath, strings.Join(args, " ")), false)
	outputCb("---", false)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readOutput(ctx, stdout, func(line string) { outputCb(line, false) })
	}()
	go func() {
		defer wg.Done()
		readOutput(ctx, stderr, func(line string) { outputCb(line, true) })
	}()
	wg.Wait()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(5 * time.Second):
		if cmd.Process != nil {
			killProcessGroup(cmd.Process.Pid)
		}
		waitErr = <-done
	}

	r.mu.Lock()
	stopped := ctx.Err() != nil
	r.cmd = nil
	r.cancel = nil
	r.mu.Unlock()

	if waitErr != nil {
		if stopped {
			r.notifyStatus(StatusStopped)
			outputCb("---", false)
			outputCb(label+"已被用户中止。", false)
		} else {
			r.notifyStatus(StatusFailed)
			outputCb("---", false)
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				outputCb(fmt.Sprintf("进程退出 %d", exitErr.ExitCode()), true)
			}
			outputCb(label+"失败："+waitErr.Error(), true)
		}
		return waitErr
	}

	r.notifyStatus(StatusCompleted)
	outputCb("---", false)
	outputCb(label+"成功完成。", false)
	return nil
}

// CancelPending 用于「任务刚提交、Runner 还没真正 Start」的时间窗口。
//
// 只有 cmd 尚未创建时才设置 stopRequested，避免影响已经在运行的任务；
// 已经在运行的任务由 Stop() 取消 context 并终止进程组。
func (r *Runner) CancelPending() {
	r.mu.Lock()
	if r.cmd == nil {
		r.stopRequested = true
	}
	r.mu.Unlock()
}

// Stop 取消正在运行的 CodeQL 进程：先发送优雅终止信号，500ms 后强制结束进程组。
func (r *Runner) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	cmd := r.cmd
	if cmd == nil && r.status == StatusRunning {
		// 任务可能刚被调度、尚未真正 Start，标记后由 run() 在启动前放弃。
		r.stopRequested = true
	}
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	terminateProcessGroup(pid)
	go func() {
		time.Sleep(500 * time.Millisecond)
		killProcessGroup(pid)
	}()
}

// Status 返回当前状态。
func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

// IsRunning 判断是否有任务正在运行。
func (r *Runner) IsRunning() bool {
	return r.Status() == StatusRunning
}

// BuildAnalyzeArgs 组装 codeql database analyze 的参数。
func (r *Runner) BuildAnalyzeArgs(config Config) []string {
	args := []string{"database", "analyze"}
	if config.DatabasePath != "" {
		args = append(args, config.DatabasePath)
	}
	if config.QueryPath != "" {
		args = append(args, config.QueryPath)
	}
	if config.OutputFormat != "" {
		args = append(args, "--format="+config.OutputFormat)
	}
	if config.OutputFile != "" {
		args = append(args, "--output="+config.OutputFile)
	}
	// 0 表示不向 CodeQL 传 --threads / --ram，使用 CodeQL 自身的默认值。
	if config.Threads > 0 {
		args = append(args, fmt.Sprintf("--threads=%d", config.Threads))
	}
	if config.RamLimit > 0 {
		args = append(args, fmt.Sprintf("--ram=%d", config.RamLimit))
	}
	args = append(args, SplitArgs(config.ExtraArgs)...)
	return args
}

// BuildDatabaseCreateArgs 组装 codeql database create 的参数。
func (r *Runner) BuildDatabaseCreateArgs(config Config) []string {
	args := []string{"database", "create"}
	if config.DatabasePath != "" {
		args = append(args, config.DatabasePath)
	}
	args = append(args, "--language="+CLILanguage(config.Language))
	if config.SourceRoot != "" {
		args = append(args, "--source-root="+config.SourceRoot)
	}
	if config.Threads > 0 {
		args = append(args, fmt.Sprintf("--threads=%d", config.Threads))
	}
	if config.RamLimit > 0 {
		args = append(args, fmt.Sprintf("--ram=%d", config.RamLimit))
	}
	args = append(args, "--overwrite")
	args = append(args, SplitArgs(config.ExtraArgs)...)
	return args
}

// CLILanguage 把界面上的语言名映射为 CodeQL CLI 实际接受的 extractor 名。
//
// CodeQL 没有独立的 typescript / kotlin extractor：
//   - TypeScript 由 javascript extractor 处理；
//   - Kotlin 由 java extractor 处理。
//
// 保留界面上的选项是为了和旧版 CodeQL Assistant 一致，但执行时必须换成
// CLI 认识的名字，否则 codeql database create 会直接报语言不支持。
func CLILanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "typescript":
		return "javascript"
	case "kotlin":
		return "java"
	default:
		return strings.TrimSpace(language)
	}
}

// SplitArgs 按空格切分参数，但双引号内的空格不会被切分。
func SplitArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := false

	for _, ch := range s {
		switch ch {
		case '"':
			inQuote = !inQuote
		case ' ', '\t', '\n':
			if inQuote {
				current.WriteRune(ch)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
