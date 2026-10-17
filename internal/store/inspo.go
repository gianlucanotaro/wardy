package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// InspoFit is a photo of someone else's outfit on the user's inspiration
// board (#49). Its tags add up to the user's style profile
// (internal/inspo). Every tag may be empty until AI or manual tagging
// fills it in; formality 0-0 means unset.
type InspoFit struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	PhotoPath    string     `json:"-"`
	ThumbPath    string     `json:"-"`
	Styles       StringList `json:"styles"`
	Colors       StringList `json:"colors"`
	Pieces       StringList `json:"pieces"`
	Fit          string     `json:"fit"`
	FormalityMin int        `json:"formality_min"`
	FormalityMax int        `json:"formality_max"`
	Seasons      StringList `json:"seasons"`
	Occasions    StringList `json:"occasions"`
	CustomTags   StringList `json:"custom_tags"`
	AITaggedAt   NullString `json:"ai_tagged_at"`
	UserEditedAt NullString `json:"user_edited_at"`
	CreatedAt    string     `json:"created_at"`
}

// InspoTags is the full set of tags for an inspo fit edit. Validation
// happens in the caller (internal/taxonomy.InspoTags) — store persists
// already-valid values, except CustomTags, which get the same cleanup as
// an item's (validateCustomTags).
type InspoTags struct {
	Styles       []string
	Colors       []string
	Pieces       []string
	Fit          string
	FormalityMin int
	FormalityMax int
	Seasons      []string
	Occasions    []string
	CustomTags   []string
}

// MaxInspoFits caps one user's board — plenty for a style profile, and a
// bound on the disk a stuck client could fill.
const MaxInspoFits = 300

// CreateInspoFit inserts a new, untagged fit for userID with its photo
// paths. Returns ErrLimitReached once the board holds MaxInspoFits.
func (s *Store) CreateInspoFit(ctx context.Context, userID int64, photoPath, thumbPath string) (int64, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM inspo_fits WHERE user_id = ?`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count inspo fits: %w", err)
	}
	if n >= MaxInspoFits {
		return 0, ErrLimitReached
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO inspo_fits (user_id, photo_path, thumb_path) VALUES (?, ?, ?)`,
		userID, photoPath, thumbPath)
	if err != nil {
		return 0, fmt.Errorf("insert inspo fit: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted id: %w", err)
	}
	return id, nil
}

// ErrLimitReached is returned when a per-user cap would be exceeded.
var ErrLimitReached = errors.New("limit reached")

const inspoColumns = `SELECT id, user_id, photo_path, thumb_path, styles, colors, pieces, fit,
	formality_min, formality_max, seasons, occasions, custom_tags, ai_tagged_at, user_edited_at, created_at`

func scanInspoFit(row rowScanner) (InspoFit, error) {
	var f InspoFit
	err := row.Scan(&f.ID, &f.UserID, &f.PhotoPath, &f.ThumbPath, &f.Styles, &f.Colors, &f.Pieces, &f.Fit,
		&f.FormalityMin, &f.FormalityMax, &f.Seasons, &f.Occasions, &f.CustomTags,
		&f.AITaggedAt, &f.UserEditedAt, &f.CreatedAt)
	if err != nil {
		return InspoFit{}, fmt.Errorf("scan inspo fit: %w", err)
	}
	return f, nil
}

// InspoFitsByUser returns userID's fits, newest first.
func (s *Store) InspoFitsByUser(ctx context.Context, userID int64) ([]InspoFit, error) {
	rows, err := s.db.QueryContext(ctx, inspoColumns+
		` FROM inspo_fits WHERE user_id = ? ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query inspo fits: %w", err)
	}
	defer rows.Close()

	fits := []InspoFit{}
	for rows.Next() {
		f, err := scanInspoFit(rows)
		if err != nil {
			return nil, err
		}
		fits = append(fits, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inspo fits: %w", err)
	}
	return fits, nil
}

// InspoFitByID returns the fit with id, but only if it belongs to userID
// — the ownership check, as for items.
func (s *Store) InspoFitByID(ctx context.Context, userID, id int64) (*InspoFit, error) {
	f, err := scanInspoFit(s.db.QueryRowContext(ctx, inspoColumns+
		` FROM inspo_fits WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// UpdateInspoTags applies a full tag edit, scoped to userID, and marks the
// fit user-edited so AI tagging never overwrites it. Returns ErrNotFound
// if the fit doesn't exist or belongs to a different user.
func (s *Store) UpdateInspoTags(ctx context.Context, userID, id int64, t InspoTags) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE inspo_fits
		SET styles = ?, colors = ?, pieces = ?, fit = ?, formality_min = ?, formality_max = ?,
		    seasons = ?, occasions = ?, custom_tags = ?,
		    user_edited_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ? AND user_id = ?`,
		StringList(t.Styles), StringList(t.Colors), StringList(t.Pieces), t.Fit, t.FormalityMin, t.FormalityMax,
		StringList(t.Seasons), StringList(t.Occasions), StringList(validateCustomTags(t.CustomTags)), id, userID)
	if err != nil {
		return fmt.Errorf("update inspo tags: %w", err)
	}
	return requireRowsAffected(res)
}

// UpdateInspoAITags applies AI tags, unless the user has edited the fit
// since upload. Custom tags are the user's own and left alone. Not
// applying (already edited, or deleted) is not an error — see
// UpdateItemAITags.
func (s *Store) UpdateInspoAITags(ctx context.Context, id int64, t InspoTags) (applied bool, err error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE inspo_fits
		SET styles = ?, colors = ?, pieces = ?, fit = ?, formality_min = ?, formality_max = ?,
		    seasons = ?, occasions = ?,
		    ai_tagged_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ? AND user_edited_at IS NULL`,
		StringList(t.Styles), StringList(t.Colors), StringList(t.Pieces), t.Fit, t.FormalityMin, t.FormalityMax,
		StringList(t.Seasons), StringList(t.Occasions), id)
	if err != nil {
		return false, fmt.Errorf("update inspo ai tags: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
}

// DeleteInspoFit removes a fit, scoped to userID, and returns its photo
// paths so the caller can remove the files. Returns ErrNotFound if the fit
// doesn't exist or belongs to a different user.
func (s *Store) DeleteInspoFit(ctx context.Context, userID, id int64) (photoPath, thumbPath string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT photo_path, thumb_path FROM inspo_fits WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&photoPath, &thumbPath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("look up inspo fit: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM inspo_fits WHERE id = ? AND user_id = ?`, id, userID); err != nil {
		return "", "", fmt.Errorf("delete inspo fit: %w", err)
	}
	return photoPath, thumbPath, nil
}

// PendingTagInspoIDs returns the ids of fits with a pending tag_inspo job,
// so the board knows which ones are still being tagged.
func (s *Store) PendingTagInspoIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT CAST(json_extract(payload, '$.fit_id') AS INTEGER)
		FROM jobs WHERE type = 'tag_inspo' AND status = 'pending'`)
	if err != nil {
		return nil, fmt.Errorf("query pending tag_inspo jobs: %w", err)
	}
	defer rows.Close()

	set := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan pending tag_inspo fit id: %w", err)
		}
		set[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending tag_inspo jobs: %w", err)
	}
	return set, nil
}
