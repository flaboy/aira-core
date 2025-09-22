package tasklib

import (
	"context"
	"encoding/json"
	"runtime/debug"
	"sync"
	"time"

	"github.com/flaboy/aira/aira-core/pkg/config"
	"github.com/flaboy/aira/aira-core/pkg/redis"

	"log/slog"

	"github.com/hibiken/asynq"
)

var (
	client    *asynq.Client
	server    *asynq.Server
	inspector *asynq.Inspector
	scheduler *asynq.Scheduler
	mux       *asynq.ServeMux
	lk        sync.Mutex
)

func init() {
	// revel.OnAppStop(StopAsynq)
	mux = asynq.NewServeMux()
	// cluster integration should be handled at the application level
}

func Init() error {
	server = asynq.NewServer(
		&redisConnector{},
		asynq.Config{
			Concurrency: 16,
			Queues: map[string]int{
				config.Config.AsynqName.Default: 2,
				config.Config.AsynqName.High:    2,
				config.Config.AsynqName.Low:     1,
			},
		},
	)

	client = asynq.NewClient(
		&redisConnector{},
	)

	inspector = asynq.NewInspector(
		&redisConnector{},
	)

	scheduler = asynq.NewScheduler(
		&redisConnector{},
		&asynq.SchedulerOpts{},
	)

	return nil
}

func Scheduler() *asynq.Scheduler {
	return scheduler
}

func StartServer() error {
	return server.Start(mux)
}

type redisConnector struct {
}

func (r *redisConnector) MakeRedisClient() interface{} {
	return redis.RedisClient
}

func StopAsynq() {
	if client != nil {
		client.Close()
	}
	if server != nil {
		server.Shutdown()
	}
	if inspector != nil {
		inspector.Close()
	}
	if scheduler != nil {
		scheduler.Shutdown()
	}
	slog.Info("Asynq stopped")
}

func Task(taskName string, v interface{}, opts ...asynq.Option) error {
	payload, err := payload(v)
	if err != nil {
		return err
	}
	task := asynq.NewTask(taskName, payload)
	_, err = client.Enqueue(task, opts...)
	return err
}

func EnqueueLowTask(taskName string, v interface{}, opts ...asynq.Option) error {
	opts = append(opts, asynq.Queue(config.Config.AsynqName.Low))
	return Task(taskName, v, opts...)
}

func EnqueueHighTask(taskName string, v interface{}, opts ...asynq.Option) error {
	opts = append(opts, asynq.Queue(config.Config.AsynqName.High))
	return Task(taskName, v, opts...)
}

func EnqueueTask(taskName string, v interface{}, opts ...asynq.Option) error {
	opts = append(opts, asynq.Queue(config.Config.AsynqName.Default))
	return Task(taskName, v, opts...)
}

func payload(v interface{}) ([]byte, error) {
	if v != nil {
		payload, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return payload, nil
	} else {
		return []byte{}, nil
	}
}

func CronTask(taskName, spec string, v interface{}, opts ...asynq.Option) (entryID string, err error) {
	payload, err := payload(v)
	if err != nil {
		return "", err
	}
	task := asynq.NewTask(taskName, payload)
	opts = append(opts, asynq.Queue(config.Config.AsynqName.Default))
	return scheduler.Register(spec, task, opts...)
}

// ScheduleTask 用于延迟任务
func ScheduleTask(taskName string, t time.Time, v interface{}, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	payload, err := payload(v)
	if err != nil {
		return nil, err
	}
	task := asynq.NewTask(taskName, payload)
	opts = append(opts, asynq.Queue(config.Config.AsynqName.Default))
	opts = append(opts, asynq.ProcessAt(t))
	return client.Enqueue(task, opts...)
}

func Consumer(taskName string, handler asynq.Handler) {
	lk.Lock()
	defer lk.Unlock()
	mux.Handle(taskName, &taskHandlerProxy{
		handler:  handler,
		taskName: taskName,
	})
}

type taskHandlerProxy struct {
	handler  asynq.Handler
	taskName string
}

func (w *taskHandlerProxy) ProcessTask(ctx context.Context, message *asynq.Task) error {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("task-error", w.taskName, "error", r, "level", "panic")
			debug.PrintStack()
		}
	}()

	err := w.handler.ProcessTask(ctx, message)
	return err
}

func Cancel(taskID string) error {
	return inspector.DeleteTask(config.Config.AsynqName.Default, taskID)
}
