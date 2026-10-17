package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// timeLayout matches the fractional-second format SQLite's own
// strftime('%Y-%m-%dT%H:%M:%fZ') default produces, so Go-written and
// SQL-defaulted timestamps compare correctly as plain strings.
const timeLayout = "2006-01-02T15:04:05.000Z"

func nowStamp() string {
	return time.Now().UTC().Format(timeLayout)
}

// maxJobAttempts is shared by the "retry once on bad output" and "3
// attempts with backoff on transport errors" policies (see docs/ai.md) —
// unified into one counter rather than two separate retry budgets.
const maxJobAttempts = 3

type Job struct {
	ID        int64
	Type      string
	Payload   string
	Status    string
	Attempts  int
	LastError string
	RunAfter  string
	CreatedAt string
}

// EnqueueJob inserts a new pending job, ready to run immediately.
func (s *Store) EnqueueJob(ctx context.Context, jobType string, payload any) (int64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal job payload: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO jobs (type, payload, run_after) VALUES (?, ?, ?)`,
		jobType, string(body), nowStamp())
	if err != nil {
		return 0, fmt.Errorf("insert job: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted id: %w", err)
	}
	return id, nil
}

// EnqueueJobOnce queues a job to run after delay, unless an identical one
// (same type and payload) is already pending — then that one's run_after
// moves to now+delay instead. Repeated triggers within the delay
// collapse into one run: a debounce, used for the style summary that
// every inspo edit would otherwise regenerate.
func (s *Store) EnqueueJobOnce(ctx context.Context, jobType string, payload any, delay time.Duration) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	runAfter := time.Now().UTC().Add(delay).Format(timeLayout)

	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET run_after = ? WHERE type = ? AND payload = ? AND status = 'pending'`,
		runAfter, jobType, string(body))
	if err != nil {
		return fmt.Errorf("reschedule pending job: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("rows affected: %w", err)
	} else if n > 0 {
		return nil
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO jobs (type, payload, run_after) VALUES (?, ?, ?)`,
		jobType, string(body), runAfter); err != nil {
		return fmt.Errorf("insert job: %w", err)
	}
	return nil
}

// HasPendingJob reports whether a job of jobType with exactly payload is
// pending — e.g. whether a user's style summary is about to be rewritten.
func (s *Store) HasPendingJob(ctx context.Context, jobType string, payload any) (bool, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("marshal job payload: %w", err)
	}
	var exists bool
	err = s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM jobs WHERE type = ? AND payload = ? AND status = 'pending')`,
		jobType, string(body)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check pending job: %w", err)
	}
	return exists, nil
}

// ClaimNextJob returns the oldest pending job of any of jobTypes whose
// run_after has passed, or ErrNotFound if there's nothing to do. There is
// exactly one worker goroutine in this app, so no locking beyond SQLite's
// own is needed to make this safe.
func (s *Store) ClaimNextJob(ctx context.Context, jobTypes ...string) (*Job, error) {
	types, err := json.Marshal(jobTypes)
	if err != nil {
		return nil, fmt.Errorf("marshal job types: %w", err)
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, payload, status, attempts, last_error, run_after, created_at
		FROM jobs
		WHERE type IN (SELECT value FROM json_each(?)) AND status = 'pending' AND run_after <= ?
		ORDER BY id
		LIMIT 1`, string(types), nowStamp())

	var j Job
	err = row.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.Attempts, &j.LastError, &j.RunAfter, &j.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("claim job: %w", err)
	}
	return &j, nil
}

// JobByID looks up a job regardless of its status — useful for tests and
// debugging, where ClaimNextJob's pending-and-due filter would hide it.
func (s *Store) JobByID(ctx context.Context, id int64) (*Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, payload, status, attempts, last_error, run_after, created_at
		FROM jobs WHERE id = ?`, id)

	var j Job
	err := row.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.Attempts, &j.LastError, &j.RunAfter, &j.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return &j, nil
}

// ItemHasPendingTagJob reports whether itemID currently has a pending
// tag_item job — used to tell the frontend whether to keep polling for a
// tagging result or stop (tagged, failed, or never queued all mean "stop").
func (s *Store) ItemHasPendingTagJob(ctx context.Context, itemID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM jobs
			WHERE type = 'tag_item' AND status = 'pending'
			AND CAST(json_extract(payload, '$.item_id') AS INTEGER) = ?
		)`, itemID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check pending tag job: %w", err)
	}
	return exists, nil
}

// PendingTagItemIDs returns the set of item ids with a pending tag_item
// job, for batch-checking a whole item list without N+1 queries.
func (s *Store) PendingTagItemIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT CAST(json_extract(payload, '$.item_id') AS INTEGER)
		FROM jobs WHERE type = 'tag_item' AND status = 'pending'`)
	if err != nil {
		return nil, fmt.Errorf("query pending tag jobs: %w", err)
	}
	defer rows.Close()

	set := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan pending tag job item id: %w", err)
		}
		set[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending tag jobs: %w", err)
	}
	return set, nil
}

// CompleteJob marks a job done.
func (s *Store) CompleteJob(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE jobs SET status = 'done' WHERE id = ?`, id); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	return nil
}

// FailJob records a failed attempt for a job currently at attempts prior
// attempts. Below maxJobAttempts it's rescheduled with exponential
// backoff (30s, 120s, ...); at the limit it's marked permanently failed —
// matching "retry, then fail the job and let the user tag manually" from
// docs/ai.md.
func (s *Store) FailJob(ctx context.Context, id int64, attempts int, errMsg string) error {
	attempts++

	if attempts >= maxJobAttempts {
		_, err := s.db.ExecContext(ctx,
			`UPDATE jobs SET status = 'failed', attempts = ?, last_error = ? WHERE id = ?`,
			attempts, errMsg, id)
		if err != nil {
			return fmt.Errorf("fail job: %w", err)
		}
		return nil
	}

	backoff := time.Duration(attempts*attempts) * 30 * time.Second
	runAfter := time.Now().UTC().Add(backoff).Format(timeLayout)

	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET attempts = ?, last_error = ?, run_after = ? WHERE id = ?`,
		attempts, errMsg, runAfter, id)
	if err != nil {
		return fmt.Errorf("reschedule job: %w", err)
	}
	return nil
}

// FailJobPermanently marks a job failed with no further retries — for
// errors retrying can't fix (monthly budget exceeded, a non-retryable
// request error).
func (s *Store) FailJobPermanently(ctx context.Context, id int64, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'failed', last_error = ? WHERE id = ?`, errMsg, id)
	if err != nil {
		return fmt.Errorf("fail job permanently: %w", err)
	}
	return nil
}
