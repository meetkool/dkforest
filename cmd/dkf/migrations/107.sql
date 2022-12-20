-- +migrate Up
ALTER TABLE links ADD COLUMN shorthand VARCHAR(50) NULL;
CREATE UNIQUE INDEX links_shorthand_uniq ON links (shorthand);

-- +migrate Down
