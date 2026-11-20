-- ASCP (AI Capability Sharing Platform) MySQL 8.0 初始化脚本
CREATE DATABASE IF NOT EXISTS `ascp` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `ascp`;

-- 1. 账号与 RBAC 权限表
CREATE TABLE IF NOT EXISTS `sys_user` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `nickname` VARCHAR(64) NOT NULL DEFAULT '',
  `email` VARCHAR(128) NOT NULL,
  `avatar` VARCHAR(255) NOT NULL DEFAULT '',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1:正常 0:禁用',
  `token_version` INT NOT NULL DEFAULT 1 COMMENT 'JWT版本号，用于登出或改密后使旧Token失效',
  `last_login_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  UNIQUE KEY `uk_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `sys_role` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `role_code` VARCHAR(32) NOT NULL COMMENT 'super_admin/domain_admin/developer',
  `role_name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_code` (`role_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `sys_user_role` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_role` (`user_id`, `role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_api_key` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `key_name` VARCHAR(64) NOT NULL,
  `key_prefix` VARCHAR(16) NOT NULL,
  `key_hash` VARCHAR(128) NOT NULL,
  `expires_at` DATETIME NULL,
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1:启用 0:吊销',
  `last_used_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_key_hash` (`key_hash`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. 组织域管理表
CREATE TABLE IF NOT EXISTS `biz_domain` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `domain_code` VARCHAR(64) NOT NULL,
  `domain_name` VARCHAR(128) NOT NULL,
  `description` VARCHAR(512) NOT NULL DEFAULT '',
  `owner_id` BIGINT UNSIGNED NOT NULL,
  `status` TINYINT NOT NULL DEFAULT 1,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_domain_code` (`domain_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `domain_member` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `domain_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `domain_role` VARCHAR(32) NOT NULL DEFAULT 'member' COMMENT 'owner/admin/member',
  `joined_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_domain_user` (`domain_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. AI 资产主表与扩展表
CREATE TABLE IF NOT EXISTS `ai_asset` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `asset_code` VARCHAR(64) NOT NULL,
  `asset_type` VARCHAR(16) NOT NULL COMMENT 'mcp/skill/agent/kb',
  `name` VARCHAR(128) NOT NULL,
  `summary` VARCHAR(512) NOT NULL DEFAULT '',
  `tags` JSON NULL,
  `domain_id` BIGINT UNSIGNED NOT NULL,
  `owner_id` BIGINT UNSIGNED NOT NULL,
  `visibility` VARCHAR(16) NOT NULL DEFAULT 'domain' COMMENT 'private/domain/public',
  `current_version` VARCHAR(32) NOT NULL DEFAULT 'v0.1.0',
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft' COMMENT 'draft/pending/published/deprecated',
  `invoke_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_asset_code` (`asset_code`),
  KEY `idx_domain_type_status` (`domain_id`, `asset_type`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ai_asset_mcp` (
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `transport_type` VARCHAR(32) NOT NULL DEFAULT 'sse' COMMENT 'sse/streamable_http/stdio',
  `endpoint_url` VARCHAR(512) NOT NULL,
  `auth_type` VARCHAR(32) NOT NULL DEFAULT 'none' COMMENT 'none/bearer/header',
  `auth_config_enc` TEXT NULL,
  `tools_schema` JSON NULL,
  `health_status` VARCHAR(16) NOT NULL DEFAULT 'unknown',
  `last_checked_at` DATETIME NULL,
  PRIMARY KEY (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ai_asset_skill` (
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `skill_type` VARCHAR(32) NOT NULL DEFAULT 'prompt' COMMENT 'prompt/script/workflow',
  `input_schema` JSON NULL,
  `output_schema` JSON NULL,
  `content` MEDIUMTEXT NULL,
  `runtime_env` VARCHAR(64) NOT NULL DEFAULT '',
  `dependencies` JSON NULL,
  PRIMARY KEY (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ai_asset_agent` (
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `model_provider` VARCHAR(64) NOT NULL DEFAULT '',
  `model_name` VARCHAR(64) NOT NULL DEFAULT '',
  `system_prompt` TEXT NULL,
  `temperature` DECIMAL(3,2) NOT NULL DEFAULT 0.70,
  `max_tokens` INT NOT NULL DEFAULT 2048,
  `bound_mcp_ids` JSON NULL,
  `bound_skill_ids` JSON NULL,
  `bound_kb_ids` JSON NULL,
  PRIMARY KEY (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ai_asset_version` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `version` VARCHAR(32) NOT NULL,
  `changelog` VARCHAR(512) NOT NULL DEFAULT '',
  `snapshot_json` JSON NOT NULL,
  `publisher_id` BIGINT UNSIGNED NOT NULL,
  `is_active` TINYINT NOT NULL DEFAULT 1,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_asset_version` (`asset_id`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4. 知识库模块表
CREATE TABLE IF NOT EXISTS `kb_knowledge_base` (
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `chunk_size` INT NOT NULL DEFAULT 500,
  `chunk_overlap` INT NOT NULL DEFAULT 50,
  `embedding_model` VARCHAR(64) NOT NULL DEFAULT '',
  `doc_count` INT NOT NULL DEFAULT 0,
  `total_chunks` INT NOT NULL DEFAULT 0,
  PRIMARY KEY (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `kb_document` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `kb_asset_id` BIGINT UNSIGNED NOT NULL,
  `file_name` VARCHAR(255) NOT NULL,
  `file_ext` VARCHAR(16) NOT NULL,
  `file_size` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `file_path` VARCHAR(512) NOT NULL,
  `file_hash` VARCHAR(128) NOT NULL DEFAULT '',
  `parse_status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/processing/completed/failed',
  `error_msg` VARCHAR(512) NOT NULL DEFAULT '',
  `chunk_count` INT NOT NULL DEFAULT 0,
  `uploader_id` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  KEY `idx_kb_asset` (`kb_asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `kb_chunk` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `kb_asset_id` BIGINT UNSIGNED NOT NULL,
  `doc_id` BIGINT UNSIGNED NOT NULL,
  `chunk_index` INT NOT NULL,
  `content` MEDIUMTEXT NOT NULL,
  `token_count` INT NOT NULL DEFAULT 0,
  `metadata` JSON NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_kb_doc` (`kb_asset_id`, `doc_id`),
  FULLTEXT KEY `ft_content` (`content`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 5. 权限、审批流与调用审计表
CREATE TABLE IF NOT EXISTS `asset_permission` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `grantee_type` VARCHAR(16) NOT NULL COMMENT 'user/domain',
  `grantee_id` BIGINT UNSIGNED NOT NULL,
  `perm_level` VARCHAR(16) NOT NULL DEFAULT 'invoke' COMMENT 'invoke/edit/admin',
  `source` VARCHAR(16) NOT NULL DEFAULT 'subscribe' COMMENT 'subscribe/approval/direct',
  `expires_at` DATETIME NULL,
  `status` VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active/revoked',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_perm_lookup` (`asset_id`, `grantee_type`, `grantee_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `approval_request` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `request_no` VARCHAR(64) NOT NULL,
  `applicant_id` BIGINT UNSIGNED NOT NULL,
  `applicant_domain_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `target_type` VARCHAR(32) NOT NULL COMMENT 'asset_access/asset_publish',
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `requested_perm` VARCHAR(16) NOT NULL DEFAULT 'invoke',
  `duration_days` INT NOT NULL DEFAULT 30,
  `reason` VARCHAR(512) NOT NULL DEFAULT '',
  `current_step` INT NOT NULL DEFAULT 1,
  `total_steps` INT NOT NULL DEFAULT 2,
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected/cancelled',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_no` (`request_no`),
  KEY `idx_applicant_status` (`applicant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `approval_record` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `request_id` BIGINT UNSIGNED NOT NULL,
  `step_no` INT NOT NULL,
  `step_name` VARCHAR(64) NOT NULL,
  `approver_id` BIGINT UNSIGNED NOT NULL,
  `action` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/approve/reject',
  `comment` VARCHAR(512) NOT NULL DEFAULT '',
  `handled_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_request_step` (`request_id`, `step_no`),
  KEY `idx_approver_action` (`approver_id`, `action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `asset_invoke_log` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `trace_id` VARCHAR(64) NOT NULL,
  `asset_id` BIGINT UNSIGNED NOT NULL,
  `asset_type` VARCHAR(16) NOT NULL,
  `asset_version` VARCHAR(32) NOT NULL DEFAULT '',
  `caller_user_id` BIGINT UNSIGNED NOT NULL,
  `caller_domain_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `auth_mode` VARCHAR(16) NOT NULL COMMENT 'jwt/api_key',
  `status_code` INT NOT NULL,
  `is_success` TINYINT NOT NULL DEFAULT 1,
  `latency_ms` INT UNSIGNED NOT NULL DEFAULT 0,
  `error_message` VARCHAR(512) NOT NULL DEFAULT '',
  `client_ip` VARCHAR(64) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_asset_time` (`asset_id`, `created_at`),
  KEY `idx_caller_time` (`caller_user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 初始系统角色种子数据
INSERT IGNORE INTO `sys_role` (`role_code`, `role_name`, `description`) VALUES
('super_admin', '平台超级管理员', '拥有全平台域、用户、资产与审批管理权限'),
('domain_admin', '域管理员', '负责所属业务域的成员管理、资产上线审核与跨域审批'),
('developer', '研发人员', '可创建与管理个人资产、检索订阅及调用授权能力');
