package loader

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Watch starts a file watcher on dir and hot-reloads any .so file that is
// created or modified. Blocks until the watcher is set up, then runs async.
func (l *Loader) Watch(dir string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return err
	}
	go func() {
		defer w.Close()
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if !strings.HasSuffix(ev.Name, ".so") {
					continue
				}
				if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
					continue
				}
				abs, _ := filepath.Abs(ev.Name)
				if err := l.Load(abs); err != nil {
					slog.Error("plugin hot-reload failed", "path", abs, "err", err)
				} else {
					slog.Info("plugin hot-reloaded", "path", abs)
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				slog.Error("plugin watcher error", "dir", dir, "err", err)
			}
		}
	}()
	return nil
}
