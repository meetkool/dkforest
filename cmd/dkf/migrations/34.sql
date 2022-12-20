-- +migrate Up
CREATE TABLE IF NOT EXISTS forum_messages_tmp (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    message TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT forum_messages_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT forum_messages_thread_id_fk
        FOREIGN KEY (thread_id)
            REFERENCES forum_threads (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

INSERT INTO forum_messages_tmp (id, uuid, message, user_id, thread_id, created_at)
SELECT id, lower(
            hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || '4' ||
            substr(hex( randomblob(2)), 2) || '-' ||
            substr('AB89', 1 + (abs(random()) % 4) , 1)  ||
            substr(hex(randomblob(2)), 2) || '-' ||
            hex(randomblob(6))
    ), message, user_id, thread_id, created_at FROM forum_messages;

DROP TABLE forum_messages;

ALTER TABLE forum_messages_tmp RENAME TO forum_messages;

CREATE INDEX forum_messages_user_id_idx ON forum_messages (user_id);
CREATE INDEX forum_messages_thread_id_idx ON forum_messages (thread_id);
-- +migrate Down
