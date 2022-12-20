-- +migrate Up
CREATE TABLE IF NOT EXISTS uploads (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    file_name VARCHAR(255) UNIQUE NOT NULL,
    orig_file_name VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL DEFAULT '',
    file_size INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uploads_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
