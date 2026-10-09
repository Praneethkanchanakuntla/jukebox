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
	ErrForbidden     = errors.New("room permission denied")
	ErrNotFound      = errors.New("room or membership not found")
	ErrInvalidRole   = errors.New("role must be co_host or listener")
	ErrHostProtected = errors.New("the host cannot be removed or demoted")
	ErrCoHostLimit   = errors.New("a room can have at most two co-hosts")
)

type ShowMembership struct {
	RoomId          int
	Member_id       string `json:"member_id"`
	Username        string `json:"username"`
	Membership_role string
	JoinedAt        time.Time
	IsOnline        bool `json:"is_online"`
}

type Store struct{ sql *sql.DB }

func NewMemebershipStore(db *sql.DB) *Store { return &Store{sql: db} }

func (store *Store) JoinRoom(ctx context.Context, roomID int, userID string) error {
	// Rejoining only changes presence; the existing role and join time remain.
	_, err := store.sql.ExecContext(ctx, `INSERT INTO membership
 (room_id, user_id, membership_role, is_online) VALUES (?, ?, 'listener', TRUE)
 ON DUPLICATE KEY UPDATE is_online = TRUE`, roomID, userID)
	if err != nil {
		return fmt.Errorf("join room: %w", err)
	}
	return nil
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

func (store *Store) ManageMembers(ctx context.Context, roomID int, targetUserID, role, actorUserID string) error {
	role = strings.TrimSpace(role)
	if role != "" && role != "co_host" && role != "listener" {
		return ErrInvalidRole
	}
	tx, err := store.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Locking the room serializes concurrent promotions and reads its owner once.
	var owner string
	err = tx.QueryRowContext(ctx, "SELECT created_by FROM room WHERE id = ? FOR UPDATE", roomID).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if actorUserID == "" || owner != actorUserID {
		return ErrForbidden
	}
	if targetUserID == owner {
		return ErrHostProtected
	}
	var current string
	err = tx.QueryRowContext(ctx, "SELECT membership_role FROM membership WHERE room_id = ? AND user_id = ? FOR UPDATE", roomID, targetUserID).Scan(&current)
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
		_, err = tx.ExecContext(ctx, "DELETE FROM membership WHERE room_id = ? AND user_id = ?", roomID, targetUserID)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE membership SET membership_role = ? WHERE room_id = ? AND user_id = ?", role, roomID, targetUserID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// SetOnline changes presence without deleting membership or losing its role.
func (store *Store) SetOnline(ctx context.Context, roomID int, userID string, online bool) error {
	result, err := store.sql.ExecContext(ctx, "UPDATE membership SET is_online = ? WHERE room_id = ? AND user_id = ?", online, roomID, userID)
	if err != nil {
		return err
	}
	// The shared MySQL pool uses ClientFoundRows, so repeated leaves still match.
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (store *Store) RequireMember(ctx context.Context, roomID int, userID string) error {
	var id int
	err := store.sql.QueryRowContext(ctx, "SELECT id FROM room WHERE id = ?", roomID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	err = store.sql.QueryRowContext(ctx, "SELECT id FROM membership WHERE room_id = ? AND user_id = ?", roomID, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrForbidden
	}
	return err
}
