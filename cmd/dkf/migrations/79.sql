-- +migrate Up
CREATE TABLE IF NOT EXISTS user_forum_thread_subscriptions (
    user_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, thread_id),
    CONSTRAINT user_forum_thread_subscriptions_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT uuser_forum_thread_subscriptions_thread_id_fk
        FOREIGN KEY (thread_id)
            REFERENCES forum_threads (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
