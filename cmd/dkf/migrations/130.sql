-- +migrate Up
CREATE INDEX chat_room_user_groups_room_id_user_id_idx ON chat_room_user_groups (room_id, user_id);

-- +migrate Down
