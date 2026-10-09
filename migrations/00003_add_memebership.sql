-- +goose Up
CREATE TABLE membership (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    room_id BIGINT UNSIGNED NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    membership_role ENUM('host', 'co_host', 'listener')
        NOT NULL DEFAULT 'listener',
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uq_membership (room_id, user_id),

    FOREIGN KEY (room_id) REFERENCES room(id)
        ON DELETE CASCADE
);
-- +goose Down
DROP TABLE IF EXISTS membership;
