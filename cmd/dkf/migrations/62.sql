-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_inbox_messages_tmp(
    id INTEGER NOT NULL PRIMARY KEY,
    message TEXT NOT NULL,
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    is_read TINYINT(1) NOT NULL DEFAULT 0,
    is_pm TINYINT(1) NOT NULL DEFAULT 0,
    chat_message_id INTEGER NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chat_inbox_messages_chat_message_id_fk
        FOREIGN KEY (chat_message_id)
            REFERENCES chat_messages (id)
            ON DELETE SET NULL
            ON UPDATE CASCADE,
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

INSERT INTO chat_inbox_messages_tmp (id, message, room_id, user_id, to_user_id, is_read, is_pm, created_at)
SELECT id, message, room_id, user_id, to_user_id, is_read, is_pm, created_at FROM chat_inbox_messages;

DROP INDEX chat_inbox_messages_to_user_id_idx;
DROP INDEX chat_inbox_messages_is_read_idx;
DROP TABLE chat_inbox_messages;

ALTER TABLE chat_inbox_messages_tmp RENAME TO chat_inbox_messages;

CREATE INDEX chat_inbox_messages_to_user_id_idx ON chat_inbox_messages (to_user_id);
CREATE INDEX chat_inbox_messages_is_read_idx ON chat_inbox_messages (is_read);
CREATE INDEX chat_inbox_messages_is_pm_idx ON chat_inbox_messages (is_pm);
CREATE INDEX chat_inbox_messages_chat_message_id_idx ON chat_inbox_messages (chat_message_id);

-- +migrate Down
