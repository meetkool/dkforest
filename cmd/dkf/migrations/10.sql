-- +migrate Up
CREATE TABLE IF NOT EXISTS gists (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) NOT NULL,
    user_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    password VARCHAR(255) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE UNIQUE INDEX gists_uuid_uniq ON gists (uuid);

-- +migrate Down
