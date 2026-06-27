package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	maxLogSize    = 10 * 1024 * 1024 // 10 MB before rotation
	maxLogBackups = 3                // keep .1, .2, .3
	logChanSize   = 8192             // buffered channel — drops entries if full
)

type pendingLog struct {
	file string
	data []byte
}

// asyncLogger writes JSON log entries to per-project files through a single
// buffered channel. This avoids synchronous file open/write/close on every
// HTTP request. Files are rotated when they exceed maxLogSize.
type asyncLogger struct {
	ch     chan pendingLog
	files  map[string]*os.File
	sizes  map[string]int64
	mu     sync.Mutex // guards files/sizes maps inside flush goroutine
	stopCh chan struct{}
	done   chan struct{}
}

var (
	reqLogger     *asyncLogger
	reqLoggerOnce sync.Once
)

// getRequestLogger returns the singleton async logger, starting its background
// flush goroutine on first call.
func getRequestLogger() *asyncLogger {
	reqLoggerOnce.Do(func() {
		reqLogger = &asyncLogger{
			ch:     make(chan pendingLog, logChanSize),
			files:  make(map[string]*os.File),
			sizes:  make(map[string]int64),
			stopCh: make(chan struct{}),
			done:   make(chan struct{}),
		}
		go reqLogger.run()
	})
	return reqLogger
}

// log queues a log entry for async writing. If the channel is full (system
// overloaded), the entry is silently dropped rather than blocking the request.
func (al *asyncLogger) log(filePath string, entry map[string]interface{}) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')
	select {
	case al.ch <- pendingLog{file: filePath, data: data}:
	default:
		// channel full — drop entry to avoid blocking the hot path
	}
}

// run is the single goroutine that drains the channel and writes to files.
func (al *asyncLogger) run() {
	defer close(al.done)
	syncTicker := time.NewTicker(5 * time.Second)
	defer syncTicker.Stop()
	for {
		select {
		case entry := <-al.ch:
			al.write(entry)
		case <-syncTicker.C:
			al.syncAll()
		case <-al.stopCh:
			// drain remaining entries
			for {
				select {
				case entry := <-al.ch:
					al.write(entry)
				default:
					al.syncAll()
					al.closeAll()
					return
				}
			}
		}
	}
}

func (al *asyncLogger) write(entry pendingLog) {
	al.mu.Lock()
	defer al.mu.Unlock()

	f, exists := al.files[entry.file]
	if !exists {
		_ = os.MkdirAll(filepath.Dir(entry.file), 0755)
		var err error
		f, err = os.OpenFile(entry.file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		if info, err := f.Stat(); err == nil {
			al.sizes[entry.file] = info.Size()
		}
		al.files[entry.file] = f
	}

	n, _ := f.Write(entry.data)
	al.sizes[entry.file] += int64(n)

	if al.sizes[entry.file] >= maxLogSize {
		al.rotate(entry.file)
	}
}

func (al *asyncLogger) rotate(path string) {
	if f, ok := al.files[path]; ok {
		_ = f.Close()
		delete(al.files, path)
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	// Shift backups: .2 -> .3, .1 -> .2, then current -> .1
	for i := maxLogBackups - 1; i >= 1; i-- {
		oldP := filepath.Join(dir, base+"."+strconv.Itoa(i))
		newP := filepath.Join(dir, base+"."+strconv.Itoa(i+1))
		_ = os.Rename(oldP, newP)
	}
	_ = os.Rename(path, filepath.Join(dir, base+".1"))
	al.sizes[path] = 0
}

// invalidate closes any cached file handle for the given path so the next
// write re-opens the file. Called by handleFlushLogs after the file is deleted.
func (al *asyncLogger) invalidate(filePath string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	if f, ok := al.files[filePath]; ok {
		_ = f.Close()
		delete(al.files, filePath)
		delete(al.sizes, filePath)
	}
}

func (al *asyncLogger) syncAll() {
	al.mu.Lock()
	defer al.mu.Unlock()
	for _, f := range al.files {
		_ = f.Sync()
	}
}

func (al *asyncLogger) closeAll() {
	al.mu.Lock()
	defer al.mu.Unlock()
	for path, f := range al.files {
		_ = f.Close()
		delete(al.files, path)
	}
}

// shutdown signals the background goroutine to drain and close.
func (al *asyncLogger) shutdown() {
	close(al.stopCh)
	<-al.done
}
