CREATE DATABASE IF NOT EXISTS homelab_ocr CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE homelab_ocr;

CREATE TABLE IF NOT EXISTS ocr_leases (
  vps_job_id BIGINT PRIMARY KEY,
  status VARCHAR(32) NOT NULL,
  last_error TEXT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  seen_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
