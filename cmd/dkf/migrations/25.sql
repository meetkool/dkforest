-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_messages_tmp (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    message TEXT NOT NULL,
    raw_message TEXT NOT NULL,
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    to_user_id INTEGER NULL,
    system TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chat_messages_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_messages_to_user_id_fk
        FOREIGN KEY (to_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_messages_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

INSERT INTO chat_messages_tmp (id, uuid, message, raw_message, room_id, user_id, to_user_id, created_at)
SELECT id, lower(
            hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || '4' ||
            substr(hex( randomblob(2)), 2) || '-' ||
            substr('AB89', 1 + (abs(random()) % 4) , 1)  ||
            substr(hex(randomblob(2)), 2) || '-' ||
            hex(randomblob(6))
    ), message, '', room_id, user_id, to_user_id, created_at FROM chat_messages;

DROP INDEX chat_messages_room_id_idx;
DROP INDEX chat_messages_user_id_idx;
DROP INDEX chat_messages_to_user_id_idx;
DROP INDEX chat_messages_created_at_idx;
DROP TABLE chat_messages;

ALTER TABLE chat_messages_tmp RENAME TO chat_messages;

CREATE INDEX chat_messages_room_id_idx ON chat_messages (room_id);
CREATE INDEX chat_messages_user_id_idx ON chat_messages (user_id);
CREATE INDEX chat_messages_to_user_id_idx ON chat_messages (to_user_id);
CREATE INDEX chat_messages_created_at_idx ON chat_messages (created_at);

-- +migrate Down
