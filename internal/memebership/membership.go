package memebership

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound      = errors.New("room or membership not found")
	ErrInvalidRole   = errors.New("role must be co_host or listener")
	ErrHostProtected = errors.New("the host cannot be removed or demoted")
	ErrCoHostLimit   = errors.New("a room can have at most two co-hosts")
)

type ShowMembership struct {
	RoomId          int
	Member_id       int
	Membership_role string
	JoinedAt        time.Time
	IsOnline        bool `json:"is_online"`
}

type Store struct{ sql *sql.DB }

func NewMemebershipStore(db *sql.DB) *Store { return &Store{sql: db} }

func (store *Store) JoinRoom(ctx context.Context, m ShowMembership) (int, error) {
	// Returning members retain their role and join time.
	result, err := store.sql.ExecContext(ctx, `INSERT INTO membership
 (room_id, user_id, membership_role, is_online) VALUES (?, ?, 'listener', TRUE)
 ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), is_online = TRUE`, m.RoomId, m.Member_id)
	if err != nil {
		return 0, fmt.Errorf("join room: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get membership id: %w", err)
	}
	return int(id), nil
}

func (store *Store) ShowMembershipDetails(ctx context.Context, roomID int) ([]ShowMembership, error) {
	rows, err := store.sql.QueryContext(ctx, `SELECT room_id, user_id, membership_role, joined_at, is_online
 FROM membership WHERE room_id = ? ORDER BY joined_at ASC`, roomID)
	if err != nil {
		return nil, fmt.Errorf("query memberships: %w", err)
	}
	defer rows.Close()
	memberships := make([]ShowMembership, 0)
	for rows.Next() {
		var m ShowMembership
		if err := rows.Scan(&m.RoomId, &m.Member_id, &m.Membership_role, &m.JoinedAt, &m.IsOnline); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships: %w", err)
	}
	return memberships, nil
}

// lockRoom serializes membership changes even when a room has no members yet.
func lockRoom(ctx context.Context, tx *sql.Tx, roomID int) error {
	var id int
	err := tx.QueryRowContext(ctx, "SELECT id FROM room WHERE id = ? FOR UPDATE", roomID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (store *Store) ManageMembers(ctx context.Context, roomID, userID int, role string) error {
	role = strings.TrimSpace(role)
	if role != "" && role != "co_host" && role != "listener" {
		return ErrInvalidRole
	}
	tx, err := store.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockRoom(ctx, tx, roomID); err != nil {
		return err
	}
	var current string
	err = tx.QueryRowContext(ctx, "SELECT membership_role FROM membership WHERE room_id = ? AND user_id = ? FOR UPDATE", roomID, userID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if current == "host" {
		return ErrHostProtected
	}
	if role == "co_host" && current != "co_host" {
		// Locking reads see promotions committed while waiting for the room lock.
		rows, err := tx.QueryContext(ctx, "SELECT user_id FROM membership WHERE room_id = ? AND membership_role = 'co_host' FOR UPDATE", roomID)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			count++
		}
		rowErr := rows.Err()
		rows.Close()
		if rowErr != nil {
			return rowErr
		}
		if count >= 2 {
			return ErrCoHostLimit
		}
	}
	if role == "" {
		_, err = tx.ExecContext(ctx, "DELETE FROM membership WHERE room_id = ? AND user_id = ?", roomID, userID)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE membership SET membership_role = ? WHERE room_id = ? AND user_id = ?", role, roomID, userID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// SetOnline changes presence without deleting membership or losing its role.
func (store *Store) SetOnline(ctx context.Context, roomID, userID int, online bool) error {
	tx, err := store.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int
	err = tx.QueryRowContext(ctx, "SELECT id FROM membership WHERE room_id = ? AND user_id = ? FOR UPDATE", roomID, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE membership SET is_online = ? WHERE id = ?", online, id); err != nil {
		return err
	}
	return tx.Commit()
}
