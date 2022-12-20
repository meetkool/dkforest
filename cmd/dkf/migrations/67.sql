-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_room_groups (
    id INTEGER NOT NULL PRIMARY KEY,
    room_id INTEGER NOT NULL,
    name VARCHAR(50) NOT NULL,
    color VARCHAR(20) NOT NULL,
    locked TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(room_id, name),
    CONSTRAINT chat_room_groups_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

CREATE TABLE IF NOT EXISTS chat_room_user_groups (
    group_id INTEGER NOT NULL,
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    PRIMARY KEY (group_id, user_id, room_id),
    CONSTRAINT chat_room_user_groups_group_id_fk
        FOREIGN KEY (group_id)
            REFERENCES chat_room_groups (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_room_user_groups_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_room_user_groups_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

ALTER TABLE chat_messages ADD COLUMN group_id INTEGER NULL;
CREATE INDEX chat_messages_group_id_idx ON chat_messages (group_id);

-- +migrate Down
