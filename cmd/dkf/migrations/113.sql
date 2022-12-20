-- +migrate Up
CREATE INDEX ignored_users_ignored_user_id_idx ON ignored_users (ignored_user_id);
CREATE INDEX pm_blacklisted_users_blacklisted_user_id_idx ON pm_blacklisted_users (blacklisted_user_id);

-- +migrate Down
