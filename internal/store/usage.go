package store

import (
	"context"
	"fmt"
	"time"
)

// LogUsage records the cost of a single model call. Every call gets
// logged regardless of outcome — cost you can't see is cost you can't
// control (docs/ai.md).
func (s *Store) LogUsage(ctx context.Context, model string, inputTokens, outputTokens int, estCostUSD float64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage (model, input_tokens, output_tokens, est_cost_usd) VALUES (?, ?, ?, ?)`,
		model, inputTokens, outputTokens, estCostUSD)
	if err != nil {
		return fmt.Errorf("log usage: %w", err)
	}
	return nil
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
