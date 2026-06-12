package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/keithy/sensible/pkg/sensible"
	"golang.org/x/sys/unix"
)

var (
	idleTimeout = flag.Duration("t", 0, "Idle timeout (e.g., 30s, 5m). 0 = daemon mode (run forever).")
	start      = flag.Bool("start", false, "Daemon mode (same as -t 0).")
	stop       = flag.Bool("stop", false, "Create stop file to stop a running consume and exit.")
	version    = flag.Bool("version", false, "Show version")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Println("sensible-consume 1.0.0")
		return
	}

	cfg := sensible.LoadConfig()
	stopFile := cfg.TasksDir + "/pending/stop"

	// Handle --stop flag
	if *stop {
		if _, err := os.Stat(stopFile); err == nil {
			fmt.Println("sensible-consume: stop file already exists")
		} else {
			if err := os.WriteFile(stopFile, []byte{}, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "sensible-consume: failed to create stop file: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("sensible-consume: stop file created")
		}
		return
	}

	// --start means daemon mode (infinite timeout)
	if *start {
		*idleTimeout = 0
	}

	storage := sensible.NewStorage(cfg.TasksDir)
	executor := sensible.NewExeExecutor()
	pendingDir := cfg.TasksDir + "/pending"

	// Main loop
	for {
		// Check for stop file first
		if _, err := os.Stat(stopFile); err == nil {
			fmt.Println("sensible-consume: stop file detected, exiting")
			os.Remove(stopFile)
			return
		}

		// Run once - process all ready tasks
		processed := runOnce(storage, executor)

		if processed > 0 {
			fmt.Printf("sensible-consume: processed %d tasks\n", processed)
		}

		// Determine watch timeout
		var watchTimeout time.Duration
		if *idleTimeout == 0 {
			// Daemon mode - watch forever (or until stop)
			watchTimeout = 30 * time.Second
		} else {
			// Timed mode - watch for idle timeout duration
			watchTimeout = *idleTimeout
		}

		// Watch for new tasks (or until stop file created)
		if err := watchForNewTasks(pendingDir, watchTimeout); err != nil {
			fmt.Printf("sensible-consume: watcher error: %v\n", err)
			return
		}

		// If timed mode and no new tasks arrived, exit
		if *idleTimeout > 0 {
			task := findReadyTask(storage)
			if task == nil {
				fmt.Println("sensible-consume: idle timeout reached, exiting")
				return
			}
		}
		// Daemon mode continues, timed mode continues if new tasks arrived
	}
}

func runOnce(storage sensible.TaskRepository, executor sensible.Executor) int {
	processed := 0
	for {
		task := findReadyTask(storage)
		if task == nil {
			if processed == 0 {
				fmt.Println("sensible-consume: no ready tasks")
			} else {
				fmt.Println("sensible-consume: no more ready tasks")
			}
			break
		}
		processTask(task, storage, executor)
		processed++
	}
	return processed
}

func findReadyTask(storage sensible.TaskRepository) *sensible.Task {
	tasks, err := storage.ListPending()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tasks: %v\n", err)
		return nil
	}

	for _, t := range tasks {
		if t.Status == "queued" {
			return t
		}
	}
	return nil
}

func processTask(task *sensible.Task, storage sensible.TaskRepository, executor sensible.Executor) {
	fmt.Printf("sensible-consume: executing %s (%s)\n", task.FileID, task.Request)

	result := executor.Execute(task.Request, 300)

	// Update task with result
	task.Status = result.Status
	task.ExitCode = result.ExitCode
	task.Stdout = result.Stdout
	task.Stderr = result.Stderr
	task.DurationMs = result.DurationMs

	// If task failed and has runNext, fail the chain
	if result.Status == "failed" && task.RunNext != "" {
		failChain(task.RunNext, storage)
	}

	// Move to done/
	if err := storage.MoveToDone(task); err != nil {
		fmt.Fprintf(os.Stderr, "Error moving to done: %v\n", err)
	}

	// Delete from pending/
	if err := storage.Delete(task.FileID); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting from pending: %v\n", err)
	}

	fmt.Printf("sensible-consume: %s -> %s (exit=%d, %dms)\n",
		task.FileID, task.Status, task.ExitCode, task.DurationMs)
}

func failChain(fileID string, storage sensible.TaskRepository) {
	next, err := storage.Load(fileID)
	if err != nil || next == nil {
		return
	}

	fmt.Printf("sensible-consume: failing chain -> %s (upstream failed)\n", next.FileID)

	next.Status = "failed"
	next.Stderr = "upstream task failed"
	storage.MoveToDone(next)
	storage.Delete(next.FileID)

	if next.RunNext != "" {
		failChain(next.RunNext, storage)
	}
}

func watchForNewTasks(dir string, timeout time.Duration) error {
	fd, err := unix.InotifyInit()
	if err != nil {
		return fmt.Errorf("inotify init: %w", err)
	}
	defer unix.Close(fd)

	wd, err := unix.InotifyAddWatch(fd, dir, unix.IN_CREATE|unix.IN_MODIFY|unix.IN_MOVED_TO)
	if err != nil {
		return fmt.Errorf("inotify add watch: %w", err)
	}
	defer unix.InotifyRmWatch(fd, uint32(wd))

	pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}

	for {
		n, err := unix.Poll(pfd, int(timeout.Milliseconds()))
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return fmt.Errorf("poll: %w", err)
		}

		if n == 0 {
			// Timeout - no new events
			return nil
		}

		// Event available - read and drain all events
		buf := make([]byte, 1024*unix.SizeofInotifyEvent)
		for {
			n, err := unix.Read(fd, buf)
			if err != nil {
				if err == unix.EAGAIN {
					break
				}
				return fmt.Errorf("read: %w", err)
			}
			if n == 0 {
				break
			}
			// Got events, return to process tasks
			return nil
		}
	}
}