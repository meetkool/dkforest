-- +migrate Up
CREATE TABLE IF NOT EXISTS ignored_users (
    user_id INTEGER NOT NULL,
    ignored_user_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, ignored_user_id),
    CONSTRAINT ignored_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT ignored_ignored_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
