CREATE TABLE IF NOT EXISTS session_summaries (
    session_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    content MEDIUMTEXT NOT NULL,
    last_message_id BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (session_id, user_id),
    INDEX idx_session_summaries_user_updated (user_id, updated_at)
);
