CREATE TABLE plan_days (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id),
	plan_date TEXT NOT NULL,
	look_id INTEGER REFERENCES looks(id) ON DELETE SET NULL,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	UNIQUE(user_id, plan_date)
);

CREATE INDEX idx_plan_days_user_date ON plan_days(user_id, plan_date);
