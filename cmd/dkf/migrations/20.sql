-- +migrate Up
ALTER TABLE users ADD COLUMN is_club_member TINYINT(1) NOT NULL DEFAULT 0;
CREATE INDEX users_is_club_idx ON users (is_club_member);

-- +migrate Down
