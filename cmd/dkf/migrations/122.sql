-- +migrate Up
CREATE TABLE IF NOT EXISTS ignored_messages (
    user_id INTEGER NOT NULL,
    message_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, message_id),
    CONSTRAINT ignored_messages_user_id_fk
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT ignored_messages_message_id_fk
        FOREIGN KEY (message_id)
        REFERENCES chat_messages (id)
        ON DELETE CASCADE
        ON UPDATE CASCADE);

-- +migrate Down
