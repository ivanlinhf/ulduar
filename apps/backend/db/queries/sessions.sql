-- name: CreateSession :one
INSERT INTO chat_sessions DEFAULT VALUES
RETURNING id, status, title, created_at, last_message_at;

-- name: GetSession :one
SELECT id, status, title, created_at, last_message_at
FROM chat_sessions
WHERE id = $1;

-- name: TouchSessionLastMessageAt :execrows
UPDATE chat_sessions
SET last_message_at = NOW()
WHERE id = $1;

-- name: SetSessionTitleIfEmpty :execrows
UPDATE chat_sessions
SET title = $2
WHERE id = $1
  AND (title IS NULL OR title = '');
