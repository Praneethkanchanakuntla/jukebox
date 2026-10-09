package room

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Room struct {
	ID                 int       `json:"id"`
	Name               string    `json:"name"`
	Locked             bool      `json:"locked"`
	Passcode           string    `json:"-"`
	CreatedAtTimeStamp time.Time `json:"created_at"`
	CreateBy           string    `json:"created_by"`
}

type CreateRoom struct {
	Name      string `form:"name" binding:"required"`
	Locked    bool   `form:"locked"`
	Passcode  string `form:"passcode"`
	CreatedBy string `json:"-"`
}

var ErrForbidden = errors.New("only the room creator can perform this action")

var ErrNotFound = errors.New("room not found")
var ErrPasscodeRequired = errors.New("locked room requires a nonempty passcode")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetByID(ctx context.Context, id int64) (*Room, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid id: %d", id)
	}

	var r Room
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, locked, COALESCE(passcode, ''), created_by, created_at FROM room WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.Locked, &r.Passcode, &r.CreateBy, &r.CreatedAtTimeStamp)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get room %d: %w", id, err)
	}
	return &r, nil
}

func (s *Store) CreateRoom(ctx context.Context, room CreateRoom) (int64, error) {
	if room.Locked && strings.TrimSpace(room.Passcode) == "" {
		return 0, ErrPasscodeRequired
	}
	if strings.TrimSpace(room.CreatedBy) == "" {
		return 0, ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO room (name,created_by,locked,passcode) values(?,?,?,?)", room.Name, room.CreatedBy, room.Locked, room.Passcode)
	if err != nil {
		return 0, fmt.Errorf("create room %q: %w", room.Name, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get new room id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO membership (room_id, user_id, membership_role, is_online) VALUES (?, ?, 'host', TRUE)", id, room.CreatedBy); err != nil {
		return 0, fmt.Errorf("create host membership: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) GetAllRooms(ctx context.Context) ([]Room, error) {
	result, err := s.db.QueryContext(ctx, "SELECT id, name, created_by, locked, COALESCE(passcode, ''), created_at FROM room")
	if err != nil {
		return []Room{}, fmt.Errorf("list rooms: %w", err)
	}
	defer result.Close()
	db_rooms := make([]Room, 0)
	for result.Next() {
		var dbRoom Room
		err := result.Scan(&dbRoom.ID, &dbRoom.Name, &dbRoom.CreateBy, &dbRoom.Locked, &dbRoom.Passcode, &dbRoom.CreatedAtTimeStamp)
		if err != nil {
			return []Room{}, fmt.Errorf("Parsing failure %w", err)
		}
		db_rooms = append(db_rooms, dbRoom)
	}
	if err = result.Err(); err != nil {
		return []Room{}, fmt.Errorf("rows iteration error: %w", err)
	}
	return db_rooms, nil

}

type UpdateRoom struct {
	Name   *string
	Locked *bool
}

func (s *Store) UpdateRoom(ctx context.Context, id int64, in UpdateRoom, userID string) (int64, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	if userID == "" || existing.CreateBy != userID {
		return 0, ErrForbidden
	}
	if in.Locked != nil && *in.Locked && strings.TrimSpace(existing.Passcode) == "" {
		return 0, ErrPasscodeRequired
	}
	result, err := s.db.ExecContext(ctx, `UPDATE room SET name = COALESCE(?, name), locked = COALESCE(?, locked)
 WHERE id = ? AND created_by = ?`, in.Name, in.Locked, id, userID)
	if err != nil {
		return 0, fmt.Errorf("update room: %w", err)
	}
	return result.RowsAffected()
}

func (s *Store) DeleteRoom(ctx context.Context, id int64, userID string) (bool, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	if userID == "" || existing.CreateBy != userID {
		return false, ErrForbidden
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM room WHERE id = ? AND created_by = ?", id, userID)
	if err != nil {
		return false, fmt.Errorf("delete room: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}
