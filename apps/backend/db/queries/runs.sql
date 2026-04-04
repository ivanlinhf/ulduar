-- name: CreateRun :one
INSERT INTO chat_runs (
    session_id,
    user_message_id,
    assistant_message_id,
    provider_response_id,
    input_tokens,
    output_tokens,
    total_tokens,
    status,
    error_code,
    started_at,
    completed_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11
)
RETURNING
    id,
    session_id,
    user_message_id,
    assistant_message_id,
    provider_response_id,
    input_tokens,
    output_tokens,
    total_tokens,
    status,
    error_code,
    started_at,
    completed_at;

-- name: GetRun :one
SELECT
    id,
    session_id,
    user_message_id,
    assistant_message_id,
    provider_response_id,
    input_tokens,
    output_tokens,
    total_tokens,
    status,
    error_code,
    started_at,
    completed_at
FROM chat_runs
WHERE id = $1;

-- name: ListRunsBySession :many
SELECT
    id,
    session_id,
    user_message_id,
    assistant_message_id,
    provider_response_id,
    input_tokens,
    output_tokens,
    total_tokens,
    status,
    error_code,
    started_at,
    completed_at
FROM chat_runs
WHERE session_id = $1
ORDER BY started_at, id;

-- name: UpdateRunState :execrows
UPDATE chat_runs
SET assistant_message_id = $2,
    provider_response_id = $3,
    input_tokens = $4,
    output_tokens = $5,
    total_tokens = $6,
    status = $7,
    error_code = $8,
    completed_at = $9
WHERE id = $1;
