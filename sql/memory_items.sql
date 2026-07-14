CREATE TABLE IF NOT EXISTS memory_items (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    memory_key VARCHAR(128) NOT NULL,
    value MEDIUMTEXT NOT NULL,
    status VARCHAR(16) NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    UNIQUE KEY uk_memory_items_identity (user_id, kind, memory_key),
    INDEX idx_memory_items_user_status_id (user_id, status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS memory_item_evidence (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    memory_item_id BIGINT NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    source_session_id VARCHAR(128) NOT NULL,
    source_message_id BIGINT NOT NULL,
    source_role VARCHAR(32) NOT NULL,
    operation VARCHAR(16) NOT NULL,
    evidence_text MEDIUMTEXT NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY uk_memory_evidence_source (memory_item_id, source_session_id, source_message_id, operation),
    INDEX idx_memory_evidence_user_created (user_id, created_at),
    CONSTRAINT fk_memory_evidence_item
        FOREIGN KEY (memory_item_id) REFERENCES memory_items (id)
        ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
