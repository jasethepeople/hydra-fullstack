package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

// Store handles all persistent storage operations
type Store struct {
	db *sql.DB
}

// NewStore creates a new SQLite store
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Info().Str("path", dbPath).Msg("sqlite store initialized")
	return store, nil
}

// migrate creates tables if they don't exist
func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS alerts (
	id TEXT PRIMARY KEY,
	alert_type TEXT NOT NULL,
	severity TEXT NOT NULL,
	category TEXT NOT NULL,
	message TEXT NOT NULL,
	value REAL,
	threshold REAL,
	unit TEXT,
	timestamp DATETIME NOT NULL,
	acknowledged INTEGER DEFAULT 0,
	acked_by TEXT,
	acked_at DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alerts_timestamp ON alerts(timestamp);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_acknowledged ON alerts(acknowledged);

CREATE TABLE IF NOT EXISTS render_jobs (
	id TEXT PRIMARY KEY,
	status TEXT NOT NULL,
	width INTEGER,
	height INTEGER,
	complexity TEXT,
	iterations INTEGER,
	duration_ms INTEGER,
	error_message TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	completed_at DATETIME,
	worker_id INTEGER
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON render_jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON render_jobs(created_at);

CREATE TABLE IF NOT EXISTS alert_notifications (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	alert_id TEXT NOT NULL,
	channel TEXT NOT NULL,
	success INTEGER DEFAULT 0,
	response_status INTEGER,
	response_body TEXT,
	latency_ms INTEGER,
	attempt_count INTEGER DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (alert_id) REFERENCES alerts(id)
);

CREATE INDEX IF NOT EXISTS idx_notifications_alert ON alert_notifications(alert_id);

CREATE TABLE IF NOT EXISTS memorial_entries (
	id TEXT PRIMARY KEY,
	entry_type TEXT NOT NULL,
	author_name TEXT,
	author_email TEXT,
	content TEXT,
	media_url TEXT,
	media_type TEXT,
	season_tag TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_memorial_type ON memorial_entries(entry_type);
CREATE INDEX IF NOT EXISTS idx_memorial_season ON memorial_entries(season_tag);

CREATE TABLE IF NOT EXISTS memorial_canvas_pixels (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	x INTEGER NOT NULL,
	y INTEGER NOT NULL,
	color TEXT NOT NULL,
	author TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_canvas_coords ON memorial_canvas_pixels(x, y);

CREATE TABLE IF NOT EXISTS system_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	event_type TEXT NOT NULL,
	severity TEXT,
	message TEXT,
	metadata TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_type ON system_events(event_type);
`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	return nil
}

// SaveAlert persists an alert to the database
func (s *Store) SaveAlert(alertID, alertType, severity, category, message string,
	value, threshold float64, unit string, timestamp time.Time) error {

	_, err := s.db.Exec(
		`INSERT INTO alerts (id, alert_type, severity, category, message, value, threshold, unit, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		alertID, alertType, severity, category, message, value, threshold, unit, timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to save alert: %w", err)
	}
	return nil
}

// GetAlertHistory retrieves paginated alert history
func (s *Store) GetAlertHistory(limit, offset int) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT id, alert_type, severity, category, message, value, threshold, unit, 
		 timestamp, acknowledged, acked_by, acked_at
		 FROM alerts ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []map[string]interface{}
	for rows.Next() {
		var id, alertType, severity, category, message, unit string
		var value, threshold float64
		var timestamp time.Time
		var acknowledged int
		var ackedBy sql.NullString
		var ackedAt sql.NullTime

		if err := rows.Scan(&id, &alertType, &severity, &category, &message, &value, &threshold, &unit,
			&timestamp, &acknowledged, &ackedBy, &ackedAt); err != nil {
			continue
		}

		alert := map[string]interface{}{
			"id":           id,
			"type":         alertType,
			"severity":     severity,
			"category":     category,
			"message":      message,
			"value":        value,
			"threshold":    threshold,
			"unit":         unit,
			"timestamp":    timestamp.Format(time.RFC3339),
			"acknowledged": acknowledged == 1,
		}
		if ackedBy.Valid {
			alert["acked_by"] = ackedBy.String
		}
		if ackedAt.Valid {
			alert["acked_at"] = ackedAt.Time.Format(time.RFC3339)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// AcknowledgeAlert updates alert acknowledgment status
func (s *Store) AcknowledgeAlert(alertID, ackedBy string) error {
	_, err := s.db.Exec(
		`UPDATE alerts SET acknowledged = 1, acked_by = ?, acked_at = ? WHERE id = ?`,
		ackedBy, time.Now().UTC(), alertID,
	)
	return err
}

// GetActiveAlertCount returns count of non-acknowledged alerts
func (s *Store) GetActiveAlertCount() (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM alerts WHERE acknowledged = 0 AND severity IN ('WARNING', 'CRITICAL')`,
	).Scan(&count)
	return count, err
}

// SaveRenderJob persists a render job
func (s *Store) SaveRenderJob(jobID, status string, width, height int, complexity string, iterations int) error {
	_, err := s.db.Exec(
		`INSERT INTO render_jobs (id, status, width, height, complexity, iterations)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		jobID, status, width, height, complexity, iterations,
	)
	return err
}

// CompleteRenderJob updates job completion
func (s *Store) CompleteRenderJob(jobID string, durationMs int, errorMsg string) error {
	status := "completed"
	if errorMsg != "" {
		status = "failed"
	}
	_, err := s.db.Exec(
		`UPDATE render_jobs SET status = ?, duration_ms = ?, error_message = ?, completed_at = ? WHERE id = ?`,
		status, durationMs, errorMsg, time.Now().UTC(), jobID,
	)
	return err
}

// SaveNotification logs a notification dispatch attempt
func (s *Store) SaveNotification(alertID, channel string, success bool, statusCode int, body string, latencyMs int64) error {
	_, err := s.db.Exec(
		`INSERT INTO alert_notifications (alert_id, channel, success, response_status, response_body, latency_ms)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		alertID, channel, success, statusCode, body, latencyMs,
	)
	return err
}

// SaveMemorialEntry persists a memorial entry
func (s *Store) SaveMemorialEntry(id, entryType, authorName, authorEmail, content, mediaURL, mediaType, seasonTag string) error {
	_, err := s.db.Exec(
		`INSERT INTO memorial_entries (id, entry_type, author_name, author_email, content, media_url, media_type, season_tag)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, entryType, authorName, authorEmail, content, mediaURL, mediaType, seasonTag,
	)
	return err
}

// GetMemorialEntries retrieves memorial entries by type and season
func (s *Store) GetMemorialEntries(entryType, seasonTag string, limit int) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT id, entry_type, author_name, content, media_url, media_type, season_tag, created_at
		 FROM memorial_entries WHERE (? = '' OR entry_type = ?) AND (? = '' OR season_tag = ?)
		 ORDER BY created_at DESC LIMIT ?`,
		entryType, entryType, seasonTag, seasonTag, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []map[string]interface{}
	for rows.Next() {
		var id, eType, author, content, mediaURL, mediaType, season string
		var createdAt time.Time
		if err := rows.Scan(&id, &eType, &author, &content, &mediaURL, &mediaType, &season, &createdAt); err != nil {
			continue
		}
		entries = append(entries, map[string]interface{}{
			"id":         id,
			"type":       eType,
			"author":     author,
			"content":    content,
			"media_url":  mediaURL,
			"media_type": mediaType,
			"season":     season,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	return entries, nil
}

// SaveCanvasPixel persists a canvas pixel
func (s *Store) SaveCanvasPixel(x, y int, color, author string) error {
	_, err := s.db.Exec(
		`INSERT INTO memorial_canvas_pixels (x, y, color, author) VALUES (?, ?, ?, ?)`,
		x, y, color, author,
	)
	return err
}

// GetCanvasPixels retrieves all canvas pixels
func (s *Store) GetCanvasPixels() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT x, y, color, author, created_at FROM memorial_canvas_pixels ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pixels []map[string]interface{}
	for rows.Next() {
		var x, y int
		var color, author string
		var createdAt time.Time
		if err := rows.Scan(&x, &y, &color, &author, &createdAt); err != nil {
			continue
		}
		pixels = append(pixels, map[string]interface{}{
			"x": x, "y": y, "color": color, "author": author,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	return pixels, nil
}

// LogSystemEvent records a system event
func (s *Store) LogSystemEvent(eventType, severity, message, metadata string) error {
	_, err := s.db.Exec(
		`INSERT INTO system_events (event_type, severity, message, metadata) VALUES (?, ?, ?, ?)`,
		eventType, severity, message, metadata,
	)
	return err
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}
