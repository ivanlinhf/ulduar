ALTER TABLE chat_runs
    ADD COLUMN input_tokens BIGINT,
    ADD COLUMN output_tokens BIGINT,
    ADD COLUMN total_tokens BIGINT;
