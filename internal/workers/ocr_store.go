package workers

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const thermalLimitMilliC = 85000

func thermalTooHot() (bool, error) {
	matches, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err != nil {
		return false, err
	}
	if len(matches) == 0 {
		return false, nil
	}
	max := 0
	for _, p := range matches {
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			continue
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if convErr != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max >= thermalLimitMilliC, nil
}

type ocrStore struct {
	db *sql.DB
}

func openOCRStore(dsn string) (*ocrStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(4)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS ocr_leases (
  vps_job_id BIGINT PRIMARY KEY,
  status VARCHAR(32) NOT NULL,
  last_error TEXT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  seen_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &ocrStore{db: db}, nil
}

func (s *ocrStore) MarkSeen(jobID int64, status string) error {
	_, err := s.db.Exec(`
INSERT INTO ocr_leases (vps_job_id, status, retry_count, seen_at, updated_at)
VALUES (?, ?, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE status=VALUES(status), updated_at=NOW()`, jobID, status)
	return err
}

func (s *ocrStore) MarkError(jobID int64, message string) error {
	_, err := s.db.Exec(`
UPDATE ocr_leases SET last_error=?, retry_count=retry_count+1, status='failed', updated_at=NOW()
WHERE vps_job_id=?`, message, jobID)
	return err
}

func (s *ocrStore) MarkDone(jobID int64) error {
	_, err := s.db.Exec(`
UPDATE ocr_leases SET status='done', last_error=NULL, updated_at=NOW() WHERE vps_job_id=?`, jobID)
	return err
}
