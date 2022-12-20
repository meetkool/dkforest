-- +migrate Up

CREATE TABLE IF NOT EXISTS downloads (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    filename VARCHAR(255) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
