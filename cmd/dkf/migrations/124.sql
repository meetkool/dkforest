-- +migrate Up
drop trigger links_before_update;
drop trigger links_before_delete;
drop trigger links_after_insert;
drop trigger links_after_update;

-- +migrate StatementBegin
CREATE TRIGGER links_before_update
    BEFORE UPDATE ON links WHEN old.deleted_at IS NULL AND new.deleted_at IS NOT NULL BEGIN
    DELETE FROM fts5_links WHERE id=old.id;
END;

CREATE TRIGGER links_before_update1
    BEFORE UPDATE ON links WHEN old.deleted_at IS NOT NULL AND new.deleted_at IS NULL BEGIN
    INSERT INTO fts5_links(rowid, uuid, url, title, description, created_at, visited_at) VALUES
        (new.id, new.uuid, new.url, new.title, new.description, new.created_at, new.visited_at);
END;
-- +migrate StatementEnd

-- +migrate Down
