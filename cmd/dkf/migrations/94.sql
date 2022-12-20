-- +migrate Up
CREATE TABLE IF NOT EXISTS user_private_notes(
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    notes BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_private_notes_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

CREATE INDEX user_private_notes_user_id_idx ON user_private_notes (user_id);

-- +migrate Down
