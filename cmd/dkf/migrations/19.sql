-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_inbox_messages (
    id INTEGER NOT NULL PRIMARY KEY,
    message TEXT NOT NULL,
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    is_read TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chat_inbox_messages_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_inbox_messages_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_inbox_messages_to_user_id_fk
        FOREIGN KEY (to_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX chat_inbox_messages_to_user_id_idx ON chat_inbox_messages (to_user_id);
CREATE INDEX chat_inbox_messages_is_read_idx ON chat_inbox_messages (is_read);

-- +migrate Down
