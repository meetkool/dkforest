-- +migrate Up
ALTER TABLE users ADD COLUMN vetted TINYINT(1) NOT NULL DEFAULT 0;

CREATE INDEX users_vetted_idx on users (vetted);

-- +migrate Down
