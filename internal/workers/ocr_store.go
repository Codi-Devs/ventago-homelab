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

const thermalLimitMilliC = 88000

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
  updated_at DATETIME NOT NULL,
  result_json LONGTEXT NULL,
  extracted_text LONGTEXT NULL
)`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &ocrStore{db: db}
	if err := store.ensureLeaseColumns(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *ocrStore) ensureLeaseColumns() error {
	alters := []string{
		`ALTER TABLE ocr_leases ADD COLUMN result_json LONGTEXT NULL`,
		`ALTER TABLE ocr_leases ADD COLUMN extracted_text LONGTEXT NULL`,
	}
	for _, q := range alters {
		if _, err := s.db.Exec(q); err != nil && !isDuplicateColumnErr(err) {
			return err
		}
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "1060")
}

func (s *ocrStore) MarkSeen(jobID int64, status string) error {
	_, err := s.db.Exec(`
INSERT INTO ocr_leases (vps_job_id, status, retry_count, seen_at, updated_at)
VALUES (?, ?, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE
  updated_at=NOW(),
  status=IF(status IN ('complete_pending','done'), status, VALUES(status))`, jobID, status)
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

func (s *ocrStore) SaveParsed(jobID int64, resultJSON string) error {
	_, err := s.db.Exec(`
INSERT INTO ocr_leases (vps_job_id, status, retry_count, seen_at, updated_at, result_json)
VALUES (?, 'complete_pending', 0, NOW(), NOW(), ?)
ON DUPLICATE KEY UPDATE status='complete_pending', result_json=VALUES(result_json), updated_at=NOW()`, jobID, resultJSON)
	return err
}

func (s *ocrStore) SaveExtracted(jobID int64, text string) error {
	_, err := s.db.Exec(`
INSERT INTO ocr_leases (vps_job_id, status, retry_count, seen_at, updated_at, extracted_text)
VALUES (?, 'ocr_text', 0, NOW(), NOW(), ?)
ON DUPLICATE KEY UPDATE extracted_text=VALUES(extracted_text), updated_at=NOW()`, jobID, text)
	return err
}

type pendingComplete struct {
	JobID      int64
	ResultJSON string
}

func (s *ocrStore) ListPendingComplete() ([]pendingComplete, error) {
	rows, err := s.db.Query(`
SELECT vps_job_id, result_json
FROM ocr_leases
WHERE status='complete_pending'
  AND result_json IS NOT NULL
  AND result_json <> ''
ORDER BY updated_at ASC
LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pendingComplete
	for rows.Next() {
		var item pendingComplete
		if err := rows.Scan(&item.JobID, &item.ResultJSON); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
