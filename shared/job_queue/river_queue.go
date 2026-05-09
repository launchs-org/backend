package job_queue

import (
	"context"
	"database/sql"

	"github.com/riverqueue/river"
)

type riverQueue struct {
	client *river.Client[*sql.Tx]
}

func (q *riverQueue) Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	_, err := q.client.Insert(ctx, args, opts)
	return err
}

func (q *riverQueue) Cancel(ctx context.Context, jobID int64) error {
	_, err := q.client.JobCancel(ctx, jobID)
	return err
}
