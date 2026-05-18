package leader

import (
	"context"
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	advisoryLockKey = int64(7482910345)
	renewInterval   = 10 * time.Second
	retryInterval   = 5 * time.Second
)

// RunWithLeaderElection acquires a PostgreSQL session-level advisory lock and
// calls fn in a child goroutine. When leadership is lost the child context is
// cancelled. Non-leader pods retry every retryInterval.
func RunWithLeaderElection(ctx context.Context, db *gorm.DB, fn func(ctx context.Context)) {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = "unknown"
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := tryAcquireLock()
		if err != nil {
			fmt.Printf("[leader] advisory lock attempt failed: %v, retrying in %s\n", err, retryInterval)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
				continue
			}
		}

		if conn == nil {
			// Lock held by another pod.
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
				continue
			}
		}

		fmt.Printf("[leader] %s became leader\n", podName)

		leaderCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})

		go func() {
			defer close(done)
			fn(leaderCtx)
		}()

		keepAlive(leaderCtx, cancel, conn, podName, done)

		// Close the dedicated connection; this releases the advisory lock so
		// another pod can become leader immediately.
		if sqlDB, err := conn.DB(); err == nil {
			sqlDB.Close()
		}

		select {
		case <-ctx.Done():
			return
		default:
			fmt.Printf("[leader] %s lost leadership, re-electing...\n", podName)
		}
	}
}

// tryAcquireLock opens a single dedicated DB connection and tries
// pg_try_advisory_lock. Returns (conn, nil) on success, (nil, nil) when the
// lock is held by another session, or (nil, err) on error.
func tryAcquireLock() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN env var not set")
	}

	conn, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Limit to a single underlying connection so the advisory lock is
	// tied to exactly one session.
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	var acquired bool
	if err := conn.Raw("SELECT pg_try_advisory_lock(?)", advisoryLockKey).Scan(&acquired).Error; err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}

	if !acquired {
		sqlDB.Close()
		return nil, nil
	}

	return conn, nil
}

// keepAlive pings the lock connection every renewInterval.
// Cancels leaderCtx when ping fails, the parent context is done, or fn returns.
func keepAlive(ctx context.Context, cancel context.CancelFunc, conn *gorm.DB, podName string, workerDone <-chan struct{}) {
	defer cancel()
	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-workerDone:
			return
		case <-ticker.C:
			if err := conn.Raw("SELECT 1").Error; err != nil {
				fmt.Printf("[leader] %s lock ping failed: %v\n", podName, err)
				return
			}
		}
	}
}
