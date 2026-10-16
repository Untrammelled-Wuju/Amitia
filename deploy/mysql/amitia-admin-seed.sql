SET NAMES utf8mb4;

CREATE DATABASE IF NOT EXISTS `amitia_admin` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE `amitia_admin`;

CREATE TABLE IF NOT EXISTS `admin_users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(512) NOT NULL,
  `role` VARCHAR(32) NOT NULL,
  `active` TINYINT(1) NOT NULL DEFAULT 1,
  `created_at` DATETIME(6) NOT NULL,
  `updated_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_admin_users_username` (`username`),
  KEY `idx_admin_users_role` (`role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `admin_sessions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `token_hash` CHAR(64) NOT NULL,
  `csrf_hash` CHAR(64) NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  `last_seen_at` DATETIME(6) NOT NULL,
  `created_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_admin_sessions_token_hash` (`token_hash`),
  KEY `idx_admin_sessions_user_id` (`user_id`),
  KEY `idx_admin_sessions_expires_at` (`expires_at`),
  CONSTRAINT `fk_admin_sessions_user` FOREIGN KEY (`user_id`) REFERENCES `admin_users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `update_releases` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `product` VARCHAR(16) NOT NULL,
  `channel` VARCHAR(16) NOT NULL,
  `version` VARCHAR(64) NOT NULL,
  `version_code` BIGINT NOT NULL DEFAULT 0,
  `status` VARCHAR(24) NOT NULL,
  `release_name` VARCHAR(160) NOT NULL DEFAULT '',
  `release_notes` LONGTEXT NOT NULL,
  `mandatory` TINYINT(1) NOT NULL DEFAULT 0,
  `min_version_code` BIGINT NOT NULL DEFAULT 0,
  `rollout_percentage` INT NOT NULL DEFAULT 100,
  `abi` VARCHAR(32) NOT NULL DEFAULT '',
  `package_name` VARCHAR(160) NOT NULL DEFAULT '',
  `publish_path` VARCHAR(512) NOT NULL DEFAULT '',
  `created_by` BIGINT UNSIGNED NOT NULL,
  `published_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL,
  `updated_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_update_release_version` (`product`, `channel`, `version`),
  KEY `idx_update_releases_status` (`status`),
  KEY `idx_update_releases_created_by` (`created_by`),
  CONSTRAINT `fk_update_releases_created_by` FOREIGN KEY (`created_by`) REFERENCES `admin_users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `update_release_artifacts` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `release_id` BIGINT UNSIGNED NOT NULL,
  `kind` VARCHAR(32) NOT NULL,
  `original_name` VARCHAR(512) NOT NULL,
  `stored_path` VARCHAR(1024) NOT NULL,
  `size` BIGINT NOT NULL,
  `sha256` CHAR(64) NOT NULL,
  `sha512` CHAR(128) NOT NULL,
  `sha512_base64` VARCHAR(256) NOT NULL,
  `created_at` DATETIME(6) NOT NULL,
  `updated_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_update_release_artifact_kind` (`release_id`, `kind`),
  CONSTRAINT `fk_update_release_artifacts_release` FOREIGN KEY (`release_id`) REFERENCES `update_releases` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `admin_audit_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `actor_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `actor_name` VARCHAR(64) NOT NULL DEFAULT '',
  `action` VARCHAR(64) NOT NULL,
  `product` VARCHAR(16) NOT NULL DEFAULT '',
  `channel` VARCHAR(16) NOT NULL DEFAULT '',
  `release_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `target` VARCHAR(512) NOT NULL DEFAULT '',
  `result` VARCHAR(24) NOT NULL,
  `message` LONGTEXT NOT NULL,
  `created_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_admin_audit_logs_actor_id` (`actor_id`),
  KEY `idx_admin_audit_logs_action` (`action`),
  KEY `idx_admin_audit_logs_product` (`product`),
  KEY `idx_admin_audit_logs_channel` (`channel`),
  KEY `idx_admin_audit_logs_release_id` (`release_id`),
  KEY `idx_admin_audit_logs_result` (`result`),
  KEY `idx_admin_audit_logs_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_feedbacks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ticket_id` VARCHAR(64) NOT NULL,
  `category` VARCHAR(32) NOT NULL DEFAULT '',
  `title` VARCHAR(200) NOT NULL DEFAULT '',
  `platform` VARCHAR(64) NOT NULL DEFAULT '',
  `client_type` VARCHAR(32) NOT NULL DEFAULT '',
  `app_version` VARCHAR(64) NOT NULL DEFAULT '',
  `core_version` VARCHAR(64) NOT NULL DEFAULT '',
  `runtime_mode` VARCHAR(32) NOT NULL DEFAULT '',
  `os_info` VARCHAR(255) NOT NULL DEFAULT '',
  `contact` VARCHAR(200) NOT NULL DEFAULT '',
  `device_hash` VARCHAR(128) NOT NULL DEFAULT '',
  `log_line_count` INT NOT NULL DEFAULT 0,
  `error_line_count` INT NOT NULL DEFAULT 0,
  `redacted_count` INT NOT NULL DEFAULT 0,
  `has_screenshot` TINYINT(1) NOT NULL DEFAULT 0,
  `screenshot_name` VARCHAR(255) NOT NULL DEFAULT '',
  `screenshot_path` VARCHAR(512) NOT NULL DEFAULT '',
  `screenshot_size` BIGINT NOT NULL DEFAULT 0,
  `client_created_at` VARCHAR(64) NOT NULL DEFAULT '',
  `payload` LONGTEXT NOT NULL,
  `received_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_feedbacks_ticket_id` (`ticket_id`),
  KEY `idx_user_feedbacks_category` (`category`),
  KEY `idx_user_feedbacks_platform` (`platform`),
  KEY `idx_user_feedbacks_device_hash` (`device_hash`),
  KEY `idx_user_feedbacks_received_at` (`received_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `admin_users` (`username`, `password_hash`, `role`, `active`, `created_at`, `updated_at`)
SELECT 'untrammelled', '$argon2id$v=19$m=65536,t=3,p=2$GPJIIGgkbro+blX+wlmzJA$FUcu8HXrR349m5ExTszYzXyNoYkjlMKY87aKihciRfA', 'admin', 1, NOW(6), NOW(6)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `admin_users` WHERE `username` = 'untrammelled');

UPDATE `admin_users`
SET `password_hash` = '$argon2id$v=19$m=65536,t=3,p=2$GPJIIGgkbro+blX+wlmzJA$FUcu8HXrR349m5ExTszYzXyNoYkjlMKY87aKihciRfA',
    `role` = 'admin',
    `active` = 1,
    `updated_at` = NOW(6)
WHERE `username` = 'untrammelled';

DELETE FROM `admin_sessions`;
