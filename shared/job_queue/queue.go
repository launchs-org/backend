package job_queue

import (
	"context"

	"github.com/riverqueue/river"
)

type Queue interface {
	Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error
	Cancel(ctx context.Context, jobID int64) error
}
