CREATE VIRTUAL TABLE fts5_forum_messages USING fts5(message, content='forum_messages', content_rowid='id');

DROP TRIGGER forum_messages_before_update;
CREATE TRIGGER forum_messages_before_update
BEFORE UPDATE ON forum_messages BEGIN
    DELETE FROM fts5_forum_messages WHERE id=old.id;
END;

DROP TRIGGER forum_messages_before_delete;
CREATE TRIGGER forum_messages_before_delete
BEFORE DELETE ON forum_messages BEGIN
    DELETE FROM fts5_forum_messages WHERE id=old.id;
END;

DROP TRIGGER forum_messages_after_update;
CREATE TRIGGER forum_messages_after_update
AFTER UPDATE ON forum_messages BEGIN
    INSERT INTO fts5_forum_messages(message)
    SELECT message
    FROM forum_messages
    WHERE new.id = forum_messages.id;
END;

DROP TRIGGER forum_messages_after_insert;
CREATE TRIGGER forum_messages_after_insert
AFTER INSERT ON forum_messages BEGIN
    INSERT INTO fts5_forum_messages(message)
    SELECT message
    FROM forum_messages
    WHERE new.id = forum_messages.id;
END;

INSERT INTO fts5_forum_messages SELECT message FROM forum_messages;

INSERT INTO fts5_forum_messages(fts5_forum_messages) VALUES('rebuild');

select highlight(fts5_forum_messages, 0, '<b>', '</b>')
from fts5_forum_messages
where fts5_forum_messages match 'spell' order by rank;