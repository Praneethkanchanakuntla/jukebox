package memebership

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ShowMembership struct {
	RoomId          int
	Member_id       int
	Membership_role string
	JoinedAt        time.Time
}

type Store struct {
	sql *sql.DB
}

func NewMemebershipStore(sql *sql.DB) *Store {
	return &Store{sql: sql}
}

func (store *Store) JoinRoom(ctx context.Context, m ShowMembership) (int, error) {
	result, err := store.sql.ExecContext(ctx, "INSERT INTO membership (room_id, user_id, membership_role) VALUES (?, ?, ?)", m.RoomId, m.Member_id, m.Membership_role)
	if err != nil {
		return 0, fmt.Errorf("unable to insert values into membership table: %w", err)
	}

	record, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("unable to fetch inserted membership id: %w", err)
	}
	return int(record), nil
}

func (store *Store) ShowMembershipDetails(ctx context.Context, roomId int) ([]ShowMembership, error) {
	// show all members in the room
	rows, err := store.sql.QueryContext(ctx, "SELECT room_id, user_id, membership_role, joined_at FROM membership WHERE room_id = ? ORDER BY joined_at ASC", roomId)
	if err != nil {
		return nil, fmt.Errorf("query memberships for room %d: %w", roomId, err)
	}
	defer rows.Close()

	memberships := make([]ShowMembership, 0)
	for rows.Next() {
		var member ShowMembership
		if err := rows.Scan(&member.RoomId, &member.Member_id, &member.Membership_role, &member.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan membership for room %d: %w", roomId, err)
		}
		memberships = append(memberships, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships for room %d: %w", roomId, err)
	}
	return memberships, nil
}

func (store *Store) ManageMembers(ctx context.Context, roomID, userID int, role string) error {
	// Manage members with a role update; an empty role removes the user from the room.
	if strings.TrimSpace(role) == "" {
		_, err := store.sql.ExecContext(ctx, "DELETE FROM membership WHERE room_id = ? AND user_id = ?", roomID, userID)
		if err != nil {
			return fmt.Errorf("remove member %d from room %d: %w", userID, roomID, err)
		}
		return nil
	}

	_, err := store.sql.ExecContext(ctx, "UPDATE membership SET membership_role = ? WHERE room_id = ? AND user_id = ?", role, roomID, userID)
	if err != nil {
		return fmt.Errorf("update membership role for user %d in room %d: %w", userID, roomID, err)
	}
	return nil
}
