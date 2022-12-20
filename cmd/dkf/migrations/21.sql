-- +migrate Up
CREATE TABLE IF NOT EXISTS forum_threads (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    user_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT forum_threads_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

CREATE TABLE IF NOT EXISTS forum_messages (
    id INTEGER NOT NULL PRIMARY KEY,
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
CREATE INDEX forum_messages_thread_id_idx ON forum_messages (thread_id);

CREATE TABLE IF NOT EXISTS forum_read_records (
    user_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    read_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, thread_id),
    CONSTRAINT forum_read_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT forum_read_thread_id_fk
        FOREIGN KEY (thread_id)
            REFERENCES forum_threads (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
-- +migrate Down
