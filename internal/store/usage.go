package store

import (
	"context"
	"fmt"
	"time"
)

// LogUsage records the cost of a single model call. Every call gets
// logged regardless of outcome — cost you can't see is cost you can't
// control (docs/ai.md). userID attributes the call for the per-user daily
// caps in DailyUsageCount.
func (s *Store) LogUsage(ctx context.Context, userID int64, model string, inputTokens, outputTokens int, estCostUSD float64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage (user_id, model, input_tokens, output_tokens, est_cost_usd) VALUES (?, ?, ?, ?, ?)`,
		userID, model, inputTokens, outputTokens, estCostUSD)
	if err != nil {
		return fmt.Errorf("log usage: %w", err)
	}
	return nil
}

// DailyUsageCount returns how many userID's calls to model were logged
// today (UTC), for the per-user daily suggestion cap (docs/ai.md: "~20
// suggestions").
func (s *Store) DailyUsageCount(ctx context.Context, userID int64, model string) (int, error) {
	dayStart := time.Now().UTC().Format("2006-01-02") + "T00:00:00.000Z"

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM usage WHERE user_id = ? AND model = ? AND created_at >= ?`,
		userID, model, dayStart,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count daily usage: %w", err)
	}
	return count, nil
}

// MonthlySpend returns the sum of estimated cost logged so far this
// calendar month (UTC), for the AI_MONTHLY_BUDGET_USD guard.
func (s *Store) MonthlySpend(ctx context.Context) (float64, error) {
	monthStart := time.Now().UTC().Format("2006-01") + "-01T00:00:00.000Z"

	var total float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(est_cost_usd), 0) FROM usage WHERE created_at >= ?`,
		monthStart,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum monthly usage: %w", err)
	}
	return total, nil
}
