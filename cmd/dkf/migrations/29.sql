-- +migrate Up
CREATE INDEX users_api_key_idx ON users (api_key);
CREATE INDEX users_is_admin_idx ON users (is_admin);
CREATE INDEX users_is_hellbanned_idx ON users (is_hellbanned);
CREATE INDEX users_verified_idx ON users (verified);

-- +migrate Down
