-- +migrate Up
ALTER TABLE users ADD COLUMN general_messages_count INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS karma_history (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    from_user_id INTEGER NULL,
    karma INTEGER NOT NULL,
    description VARCHAR(255) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT karma_history_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT karma_history_from_user_id_fk
        FOREIGN KEY (from_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX karma_history_user_id_idx ON karma_history (user_id);

-- +migrate Down
