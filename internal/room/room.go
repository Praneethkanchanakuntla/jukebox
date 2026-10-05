package room

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	Name     string `form:"name" binding:"required"`
	Locked   bool   `form:"locked"`
	Passcode string `form:"passcode"`
}

var ErrNotFound = errors.New("room not found")

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
		`SELECT id, name, locked, passcode FROM room WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.Locked, &r.Passcode)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get room %d: %w", id, err)
	}
	return &r, nil
}

func (s *Store) CreateRoom(ctx context.Context, room CreateRoom) (int64, error) {
	result, err := s.db.ExecContext(ctx, "INSERT INTO room (name,created_by,locked,passcode) values(?,?,?,?)", room.Name, "sample", room.Locked, room.Passcode)
	if err != nil {
		return 0, fmt.Errorf("create room %q: %w", room.Name, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get new room id: %w", err)
	}

	return id, nil
}

func (s *Store) GetAllRooms(ctx context.Context) ([]Room, error) {
	result, err := s.db.QueryContext(ctx, "select * from room")
	if err != nil {
		return []Room{}, fmt.Errorf("create room %w", err)
	}
	defer result.Close()
	var db_rooms []Room
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
