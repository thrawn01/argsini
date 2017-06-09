package argsini

import (
	"sync"
	"time"

	"context"

	"github.com/pkg/errors"
	"gopkg.in/fsnotify.v1"
)

type fileEvent struct {
	Err   error
	Event *fsnotify.Event
}

func watchFile(ctx context.Context, path string, interval time.Duration) (chan fileEvent, error) {
	var isRunning sync.WaitGroup

	fsWatch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "while creating new fsnotify watcher")
	}

	if err := fsWatch.Add(path); err != nil {
		return nil, errors.Wrapf(err, "while adding '%s' to watch list", path)
	}

	// Check for write events at this interval
	tick := time.Tick(interval)
	result := make(chan fileEvent, 0)
	once := sync.Once{}

	isRunning.Add(1)
	go func() {
		var lastWriteEvent *fsnotify.Event
		var checkFile *fsnotify.Event

		for {
			once.Do(func() { isRunning.Done() }) // Notify we are running
			select {
			case event := <-fsWatch.Events:
				// If it was a write event
				if event.Op&fsnotify.Write == fsnotify.Write {
					lastWriteEvent = &event
				}
				// VIM apparently renames a file before writing
				if event.Op&fsnotify.Rename == fsnotify.Rename {
					checkFile = &event
				}
				// If we see a Remove event, This is probably ConfigMap updating the config symlink
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					checkFile = &event
				}
			case <-tick:
				// If the file was renamed or removed; maybe it re-appears after our duration?
				if checkFile != nil {
					// Since the file was removed, we must
					// re-register the file to be watched
					fsWatch.Remove(checkFile.Name)
					if err := fsWatch.Add(checkFile.Name); err != nil {
						// Nothing left to watch
						result <- fileEvent{
							Event: nil,
							Err:   errors.Wrap(err, "watched file disappeared")}
						return
					}
					lastWriteEvent = checkFile
					checkFile = nil
					continue
				}

				// No events during this interval
				if lastWriteEvent == nil {
					continue
				}
				result <- fileEvent{lastWriteEvent}
				// Reset the last event
				lastWriteEvent = nil
			case ctx.Done():
				return
			}
		}
	}()

	// Wait until the go-routine is running before we return, this ensures we
	// pickup any file changes after we leave this function
	isRunning.Wait()

	return result, nil
}
