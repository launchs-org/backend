package job_queue

import (
	"context"
	"database/sql"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
)

var Default Queue

// UseRiver は River 実装で Queue を初期化する。
// sqlDB: taskdb への *sql.DB 接続
// workers: ワーカー登録済みの *river.Workers（nil なら挿入専用クライアント）
// queues: 処理するキュー名のリスト（nil の場合は default キューのみ）
func UseRiver(ctx context.Context, sqlDB *sql.DB, workers *river.Workers, queues ...string) error {
	migrator, err := rivermigrate.New(riverdatabasesql.New(sqlDB), nil)
	if err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}

	cfg := &river.Config{}
	if workers != nil {
		queueConfig := map[string]river.QueueConfig{}
		if len(queues) == 0 {
			// キュー名が未指定の場合のみ default キューを購読する
			queueConfig[river.QueueDefault] = river.QueueConfig{MaxWorkers: 10}
		}
		for _, q := range queues {
			queueConfig[q] = river.QueueConfig{MaxWorkers: 10}
		}
		cfg.Queues = queueConfig
		cfg.Workers = workers
	}

	client, err := river.NewClient(riverdatabasesql.New(sqlDB), cfg)
	if err != nil {
		return err
	}

	if workers != nil {
		if err := client.Start(ctx); err != nil {
			return err
		}
	}

	Default = &riverQueue{client: client}
	return nil
}

func Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	return Default.Enqueue(ctx, args, opts)
}

// EnqueueTo は opts の Queue フィールドを上書きしてジョブを投入します。
// InsertOpts() メソッドより確実にキューを指定したい場合に使います。
func EnqueueTo(ctx context.Context, queue string, args river.JobArgs, opts *river.InsertOpts) error {
	if opts == nil {
		opts = &river.InsertOpts{}
	}
	opts.Queue = queue
	return Default.Enqueue(ctx, args, opts)
}

func Cancel(ctx context.Context, jobID int64) error {
	return Default.Cancel(ctx, jobID)
}
