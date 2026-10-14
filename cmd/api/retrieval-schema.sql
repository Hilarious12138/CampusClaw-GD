CREATE OR REPLACE VIEW knowledge_entries AS SELECT material_id AS id, material_id, body AS body_text FROM material_contents;
CREATE TABLE IF NOT EXISTS material_indexes (
 material_id CHAR(32) PRIMARY KEY,
 version INT NOT NULL DEFAULT 1,
 options JSON NOT NULL,
 state ENUM('pending','indexing','ready','failed') NOT NULL DEFAULT 'pending',
 processed_text LONGTEXT NOT NULL,
 offset_basis ENUM('original','processed') NOT NULL DEFAULT 'original',
 attempts INT NOT NULL DEFAULT 0,
 next_attempt DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 last_error VARCHAR(255) NOT NULL DEFAULT '',
 FOREIGN KEY(material_id) REFERENCES material_contents(material_id) ON DELETE CASCADE,
 INDEX(state,next_attempt)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS knowledge_chunks (
 id CHAR(36) PRIMARY KEY,
 material_id CHAR(32) NOT NULL,
 knowledge_entry_id CHAR(32) NOT NULL,
 class_id CHAR(32) NOT NULL,
 version INT NOT NULL,
 chunk_index INT NOT NULL,
 start_char INT NOT NULL,
 end_char INT NOT NULL,
 offset_basis ENUM('original','processed') NOT NULL,
 chunk_text TEXT NOT NULL,
 index_status ENUM('pending','ready','failed') NOT NULL DEFAULT 'pending',
 FOREIGN KEY(material_id) REFERENCES material_contents(material_id) ON DELETE CASCADE,
 FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE CASCADE,
 UNIQUE KEY(material_id,version,chunk_index),
 INDEX(class_id,index_status),
 FULLTEXT KEY chunk_fulltext(chunk_text) WITH PARSER ngram
) ENGINE=InnoDB;
