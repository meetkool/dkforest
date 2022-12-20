-- +migrate Up
CREATE VIRTUAL TABLE fts5_forum_messages USING fts5(id UNINDEXED, uuid UNINDEXED, thread_id UNINDEXED, message, content='forum_messages', content_rowid='id');

CREATE VIRTUAL TABLE fts5_forum_threads USING fts5(id UNINDEXED, uuid UNINDEXED, name, content='forum_threads', content_rowid='id');

-- +migrate StatementBegin
CREATE TRIGGER forum_messages_before_update
    BEFORE UPDATE ON forum_messages BEGIN
    DELETE FROM fts5_forum_messages WHERE id=old.id;
END;

CREATE TRIGGER forum_messages_before_delete
    BEFORE DELETE ON forum_messages BEGIN
    DELETE FROM fts5_forum_messages WHERE id=old.id;
END;

CREATE TRIGGER forum_messages_after_update
    AFTER UPDATE ON forum_messages BEGIN
    INSERT INTO fts5_forum_messages(id, uuid, thread_id, message)
    SELECT id, uuid, thread_id, message
    FROM forum_messages
    WHERE new.id = forum_messages.id;
END;

CREATE TRIGGER forum_messages_after_insert
    AFTER INSERT ON forum_messages BEGIN
    INSERT INTO fts5_forum_messages(id, uuid, thread_id, message)
    SELECT id, uuid, thread_id, message
    FROM forum_messages
    WHERE new.id = forum_messages.id;
END;

CREATE TRIGGER forum_threads_before_update
    BEFORE UPDATE ON forum_threads BEGIN
    DELETE FROM fts5_forum_threads WHERE id=old.id;
END;

CREATE TRIGGER forum_threads_before_delete
    BEFORE DELETE ON forum_threads BEGIN
    DELETE FROM fts5_forum_threads WHERE id=old.id;
END;

CREATE TRIGGER forum_threads_after_update
    AFTER UPDATE ON forum_threads BEGIN
    INSERT INTO fts5_forum_threads(id, uuid, name)
    SELECT id, uuid, name
    FROM forum_threads
    WHERE new.id = forum_threads.id;
END;

CREATE TRIGGER forum_threads_after_insert
    AFTER INSERT ON forum_threads BEGIN
    INSERT INTO fts5_forum_threads(id, uuid, name)
    SELECT id, uuid, name
    FROM forum_threads
    WHERE new.id = forum_threads.id;
END;
-- +migrate StatementEnd

INSERT INTO fts5_forum_threads SELECT id, uuid, name FROM forum_threads;
INSERT INTO fts5_forum_threads(fts5_forum_threads) VALUES('rebuild');
INSERT INTO fts5_forum_messages SELECT id, uuid, thread_id, message FROM forum_messages;
INSERT INTO fts5_forum_messages(fts5_forum_messages) VALUES('rebuild');

-- +migrate Down
