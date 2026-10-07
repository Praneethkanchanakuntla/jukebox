-- +goose Up
ALTER TABLE membership
ADD COLUMN is_online BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE membership DROP COLUMN is_online;
