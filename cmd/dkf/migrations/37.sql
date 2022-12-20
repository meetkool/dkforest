-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_read_markers (
    user_id INTEGER NOT NULL,
    room_id INTEGER NOT NULL,
    read_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, room_id),
    CONSTRAINT chat_read_markers_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_read_markers_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

ALTER TABLE users ADD COLUMN chat_read_marker_enabled TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
