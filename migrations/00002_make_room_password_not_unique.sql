-- +goose Up
ALTER TABLE room DROP INDEX passcode;

-- +goose Down
ALTER TABLE room ADD UNIQUE (passcode);
