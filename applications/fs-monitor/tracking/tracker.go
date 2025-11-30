package tracking

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type Tracker struct {
	watcher *fsnotify.Watcher
}

func NewTracker(dirPath string) (*Tracker, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err = addDirRecursive(watcher, dirPath); err != nil {
		return nil, err
	}

	return &Tracker{watcher: watcher}, nil
}

func addDirRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := watcher.Add(path); err != nil {
				return err
			}
			slog.Info("Watching directory", "path", path)
		}
		return nil
	})
}

func (t *Tracker) Run() {
	for {
		select {
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			t.handleEvent(event)

		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func (t *Tracker) handleEvent(event fsnotify.Event) {
	slog.Info("Event detected", "event", event)
}
