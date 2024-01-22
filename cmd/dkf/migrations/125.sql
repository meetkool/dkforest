-- +migrate Up
drop trigger links_before_update;
drop trigger links_before_update1;

-- +migrate StatementBegin
CREATE TRIGGER links_before_update_soft_delete
    BEFORE UPDATE ON links WHEN old.deleted_at IS NULL AND new.deleted_at IS NOT NULL BEGIN
    DELETE FROM fts5_links WHERE id=old.id;
END;

CREATE TRIGGER links_before_update
    BEFORE UPDATE ON links WHEN old.deleted_at IS NOT NULL AND new.deleted_at IS NULL BEGIN
    INSERT INTO fts5_links(rowid, uuid, url, title, description, created_at, visited_at) VALUES
        (new.id, new.uuid, new.url, new.title, new.description, new.created_at, new.visited_at);
END;

CREATE TRIGGER links_after_insert
    AFTER INSERT ON links BEGIN
    INSERT INTO fts5_links(rowid, uuid, url, title, description, created_at, visited_at)
        SELECT id, uuid, url, title, description, created_at, visited_at
        FROM links
        WHERE links.id = new.id;
END;
-- +migrate StatementEnd

-- +migrate Down
