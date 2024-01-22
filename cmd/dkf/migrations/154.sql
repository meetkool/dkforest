-- +migrate Up
ALTER TABLE poker_casino ADD COLUMN hands_played INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
