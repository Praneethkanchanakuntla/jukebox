package corn

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DeleteStaleRooms(ctx context.Context) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // no-op after a successful Commit

	res, err := tx.ExecContext(ctx, `
		DELETE FROM room
		WHERE NOT EXISTS (
			SELECT 1 FROM membership m
			WHERE m.room_id = room.id
			  AND m.user_id = room.created_by
			  AND m.is_online = TRUE
			  AND m.last_updated_at > CURRENT_TIMESTAMP - INTERVAL 90 SECOND
		)`)
	if err != nil {
		return 0, fmt.Errorf("delete stale rooms: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}
