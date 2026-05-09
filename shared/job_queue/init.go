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
func UseRiver(ctx context.Context, sqlDB *sql.DB, workers *river.Workers) error {
	migrator, err := rivermigrate.New(riverdatabasesql.New(sqlDB), nil)
	if err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}

	cfg := &river.Config{}
	if workers != nil {
		cfg.Queues = map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		}
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

func Cancel(ctx context.Context, jobID int64) error {
	return Default.Cancel(ctx, jobID)
}
