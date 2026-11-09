-- +goose Up
ALTER TABLE room MODIFY COLUMN passcode VARCHAR(255) NULL;

-- +goose Down
ALTER TABLE room MODIFY COLUMN passcode VARCHAR(10) NULL;
