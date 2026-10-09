CREATE DATABASE IF NOT EXISTS offline_rag
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_0900_ai_ci;

USE offline_rag;

SOURCE /schema/recentchat_messages.sql;
SOURCE /schema/session_summaries.sql;
SOURCE /schema/memory_items.sql;
SOURCE /schema/document_ingestion.sql;
