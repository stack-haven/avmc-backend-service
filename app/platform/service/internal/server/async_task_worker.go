package server

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/platform/service/internal/biz"
	entviewer "backend-service/app/platform/service/internal/data/ent/viewer"
)

type AsyncTaskWorker struct {
	uc       *biz.AsyncTaskUsecase
	log      *log.Helper
	workerID string
	stop     chan struct{}
	once     sync.Once
	nextPoll time.Time
	backoff  time.Duration
}

func NewAsyncTaskWorker(uc *biz.AsyncTaskUsecase, logger log.Logger) *AsyncTaskWorker {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return &AsyncTaskWorker{
		uc:       uc,
		log:      log.NewHelper(log.With(logger, "module", "server/async-task-worker")),
		workerID: fmt.Sprintf("%s:%d", hostname, os.Getpid()),
		stop:     make(chan struct{}),
	}
}

func (w *AsyncTaskWorker) Start(ctx context.Context) error {
	w.log.Infof("async task worker started: worker=%s", w.workerID)
	systemCtx := entviewer.NewSystemContext(ctx)
	if err := w.uc.EnsureMaintenanceTasks(systemCtx, time.Now()); err != nil {
		w.log.Errorf("ensure maintenance tasks: %v", err)
	}

	pollTicker := time.NewTicker(500 * time.Millisecond)
	maintenanceTicker := time.NewTicker(time.Hour)
	defer pollTicker.Stop()
	defer maintenanceTicker.Stop()

	for {
		select {
		case <-w.stop:
			w.log.Info("async task worker stopped")
			return nil
		case <-ctx.Done():
			return nil
		case now := <-maintenanceTicker.C:
			if err := w.uc.EnsureMaintenanceTasks(systemCtx, now); err != nil {
				w.log.Errorf("ensure maintenance tasks: %v", err)
			}
		case now := <-pollTicker.C:
			if now.Before(w.nextPoll) {
				continue
			}
			if err := w.drain(systemCtx); err != nil {
				if w.backoff == 0 {
					w.backoff = time.Second
				} else {
					w.backoff = min(w.backoff*2, 30*time.Second)
				}
				w.nextPoll = now.Add(w.backoff)
				w.log.Errorf("async task polling paused for %s: %v", w.backoff, err)
			} else {
				w.backoff = 0
				w.nextPoll = time.Time{}
			}
		}
	}
}

func (w *AsyncTaskWorker) Stop(context.Context) error {
	w.once.Do(func() { close(w.stop) })
	return nil
}

func (w *AsyncTaskWorker) drain(ctx context.Context) error {
	for _, queue := range []string{"default", "maintenance", "notification", "webhook"} {
		for i := 0; i < 10; i++ {
			handled, err := w.uc.RunOne(ctx, w.workerID, queue, 2*time.Minute)
			if err != nil {
				return fmt.Errorf("queue %s: %w", queue, err)
			}
			if !handled {
				break
			}
		}
	}
	return nil
}
