-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_messages_tmp (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    message TEXT NOT NULL,
    raw_message TEXT NOT NULL,
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    upload_id INTEGER NULL,
    to_user_id INTEGER NULL,
    system TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_hellbanned TINYINT(1) NOT NULL DEFAULT 0,
    moderators TINYINT(1) NOT NULL DEFAULT 0,
    group_id INTEGER NULL,
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
            ON UPDATE CASCADE,
    CONSTRAINT chat_messages_upload_id_fk
        FOREIGN KEY (upload_id)
            REFERENCES uploads (id)
            ON DELETE SET NULL
            ON UPDATE CASCADE);

INSERT INTO chat_messages_tmp (id, uuid, message, raw_message, room_id, user_id, to_user_id, system, created_at, is_hellbanned, moderators, group_id)
SELECT id, uuid, message, raw_message, room_id, user_id, to_user_id, system, created_at, is_hellbanned, moderators, group_id FROM chat_messages;

DROP INDEX chat_messages_room_id_idx;
DROP INDEX chat_messages_user_id_idx;
DROP INDEX chat_messages_group_id_idx;
DROP INDEX chat_messages_is_hellbanned_idx;
DROP INDEX chat_messages_moderators_idx;
DROP INDEX chat_messages_to_user_id_idx;
DROP INDEX chat_messages_created_at_idx;
DROP TABLE chat_messages;

ALTER TABLE chat_messages_tmp RENAME TO chat_messages;

CREATE INDEX chat_messages_room_id_idx ON chat_messages (room_id);
CREATE INDEX chat_messages_user_id_idx ON chat_messages (user_id);
CREATE INDEX chat_messages_to_user_id_idx ON chat_messages (to_user_id);
CREATE INDEX chat_messages_created_at_idx ON chat_messages (created_at);
CREATE INDEX chat_messages_group_id_idx ON chat_messages (group_id);
CREATE INDEX chat_messages_is_hellbanned_idx ON chat_messages (is_hellbanned);
CREATE INDEX chat_messages_moderators_idx ON chat_messages (moderators);

-- +migrate Down
