-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_reactions (
    user_id INTEGER NOT NULL,
    message_id INTEGER NOT NULL,
    reaction INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, message_id, reaction),
    CONSTRAINT chat_reactions_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_reactions_message_id_fk
        FOREIGN KEY (message_id)
            REFERENCES chat_messages (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
