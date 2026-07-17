CREATE TABLE IF NOT EXISTS document_sources (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    knowledge_scope VARCHAR(128) NOT NULL,
    document_id VARCHAR(128) NOT NULL,
    source_ref VARCHAR(1024) NOT NULL,
    active_version_id BIGINT NULL COMMENT 'Validated against this source inside the activation transaction',
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_document_source (knowledge_scope, document_id),
    KEY idx_document_source_active_version (active_version_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS document_versions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    document_source_id BIGINT NOT NULL,
    content_hash CHAR(64) NOT NULL,
    parser_version VARCHAR(128) NOT NULL,
    chunk_policy_hash CHAR(64) NOT NULL,
    status ENUM('pending', 'building', 'ready', 'active', 'failed') NOT NULL,
    target_collection VARCHAR(255) NOT NULL,
    chunk_count INT UNSIGNED NOT NULL DEFAULT 0,
    error_message VARCHAR(2048) NOT NULL DEFAULT '',
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    activated_at TIMESTAMP(6) NULL,
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_document_version_source
        FOREIGN KEY (document_source_id) REFERENCES document_sources(id) ON DELETE RESTRICT,
    UNIQUE KEY uq_document_version_build
        (document_source_id, content_hash, parser_version, chunk_policy_hash, target_collection),
    KEY idx_document_version_status (status, updated_at),
    KEY idx_document_version_source_status (document_source_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS document_chunk_manifests (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    document_version_id BIGINT NOT NULL,
    chunk_id CHAR(64) NOT NULL,
    structure_kind VARCHAR(64) NOT NULL,
    heading_path VARCHAR(1024) NOT NULL,
    ordinal INT UNSIGNED NOT NULL,
    content_hash CHAR(64) NOT NULL,
    token_count INT UNSIGNED NOT NULL,
    qdrant_point_id CHAR(36) NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_document_chunk_version
        FOREIGN KEY (document_version_id) REFERENCES document_versions(id) ON DELETE CASCADE,
    UNIQUE KEY uq_document_version_chunk (document_version_id, chunk_id),
    UNIQUE KEY uq_document_version_ordinal (document_version_id, ordinal),
    KEY idx_document_chunk_point (qdrant_point_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
