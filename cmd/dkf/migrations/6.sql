-- +migrate Up

CREATE TABLE IF NOT EXISTS chat_rooms_tmp (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    owner_user_id INTEGER NULL,
    password VARCHAR(255) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chat_rooms_owner_user_id_fk
        FOREIGN KEY (owner_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

INSERT INTO chat_rooms_tmp (id, name, created_at)
SELECT id, name, created_at FROM chat_rooms;

DROP TABLE chat_rooms;

ALTER TABLE chat_rooms_tmp RENAME TO chat_rooms;

CREATE INDEX chat_rooms_created_at_idx ON chat_rooms (created_at);

-- +migrate Down
