package api

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jukebox/internal/room"

	"jukebox/internal/memebership"

	"jukebox/internal/config"

	"jukebox/internal/middleware"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/gin-gonic/gin"
)

func RegisterRoomEndpoints(route *gin.Engine, db *sql.DB) {
	store := room.NewStore(db)
	//load clerk to keep as middle ware
	config.LoadClerkCereds()

	memebership := memebership.NewMemebershipStore(db)

	roomGroup := route.Group("/room")
	roomGroup.Use(middleware.ClerkAuthMiddleware())
	roomGroup.GET("/openRoom/:id", func(ctx *gin.Context) {
		OpenRoom(ctx, store, memebership)
	})
	roomGroup.POST("/createRoom", func(ctx *gin.Context) {
		CreateRooms(ctx, store)
	})
	roomGroup.GET("/getAllRooms", func(ctx *gin.Context) {
		GetRooms(ctx, store)
	})
	roomGroup.PUT("/editRoom/:id", func(ctx *gin.Context) {
		editRoom(ctx, store)
	})
	roomGroup.DELETE("/delete/:id", func(ctx *gin.Context) {
		DeleteRoom(ctx, store)
	})
	roomGroup.POST("/:id/leave", func(ctx *gin.Context) {
		LeaveRoom(ctx, memebership)
	})
	roomGroup.GET("/:id/members", func(ctx *gin.Context) {
		ShowMembersInroom(ctx, memebership)
	})
	roomGroup.PUT("/:id/members/:userId", func(ctx *gin.Context) {
		ManagePeople(ctx, memebership)
	})
	roomGroup.DELETE("/:id/members/:userId", func(ctx *gin.Context) {
		ManagePeople(ctx, memebership)
	})
}

func CreateRooms(ctx *gin.Context, store *room.Store) {
	var roomReq room.CreateRoom
	if err := ctx.ShouldBind(&roomReq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	clerkId := ctx.GetString("clerk_user_id")
	roomReq.CreatedBy = clerkId
	roomId, err := store.CreateRoom(ctx.Request.Context(), roomReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	username := "Listener"
	profile, profileErr := user.Get(ctx.Request.Context(), clerkId)
	if profileErr == nil {
		username = displayName(profile)
	} else {
		log.Printf("load creator profile: %v", profileErr)
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"id":       roomId,
		"name":     roomReq.Name,
		"username": username,
	})
}

func GetRooms(ctx *gin.Context, store *room.Store) {
	rooms, err := store.GetAllRooms(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get rooms"})
		return
	}
	ids := make([]string, 0, len(rooms))
	for _, r := range rooms {
		ids = append(ids, r.CreateBy)
	}
	names := clerkDisplayNames(ctx.Request.Context(), ids)
	response := make([]gin.H, 0, len(rooms))
	for _, r := range rooms {
		name := names[r.CreateBy]
		if name == "" {
			name = "Listener"
		}
		response = append(response, gin.H{
			"id": r.ID, "name": r.Name, "locked": r.Locked,
			"created_at": r.CreatedAtTimeStamp, "created_by": name,
		})
	}
	ctx.JSON(http.StatusOK, response)
}

type updateRoomRequest struct {
	Name   *string `json:"name"`
	Locked *bool   `json:"locked"`
}

func editRoom(c *gin.Context, store *room.Store) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	var req updateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Name == nil && req.Locked == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
		return
	}
	clerk_id := c.GetString("clerk_user_id")
	rows, err := store.UpdateRoom(c.Request.Context(), id, room.UpdateRoom{
		Name:   req.Name,
		Locked: req.Locked,
	}, clerk_id)
	if errors.Is(err, room.ErrForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, room.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, room.ErrPasscodeRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Printf("failed to update room %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update room"})
		return
	}
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "room updated"})
}
func OpenRoom(ctx *gin.Context, store *room.Store, member *memebership.Store) *room.Room {
	roomParam := ctx.Param("id")
	roomId, err := strconv.ParseInt(roomParam, 10, 64)
	if err != nil || roomId <= 0 {
		ctx.JSON(400, gin.H{"error": "invalid room id"})
		return nil
	}

	passCode := ctx.Query("passCode")

	dbRoom, err := store.GetByID(ctx.Request.Context(), roomId)
	if errors.Is(err, room.ErrNotFound) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return nil
	}
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return nil
	}

	if dbRoom.Locked {
		if dbRoom.Passcode != passCode {
			ctx.JSON(401, gin.H{"error": "wrong passcode"})
			return nil
		}
	}

	userID := ctx.GetString("clerk_user_id")
	if err := member.JoinRoom(ctx.Request.Context(), int(roomId), userID); err != nil {
		log.Printf("join room %d: %v", roomId, err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join room"})
		return nil
	}
	ctx.JSON(200, dbRoom)
	return dbRoom
}

func DeleteRoom(ctx *gin.Context, store *room.Store) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}
	clerk_id := ctx.GetString("clerk_user_id")

	deleted, err := store.DeleteRoom(ctx.Request.Context(), id, clerk_id)
	if errors.Is(err, room.ErrForbidden) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, room.ErrNotFound) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete room"})
		return
	}
	if !deleted {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}
	ctx.JSON(http.StatusOK, deleted)
}

func ManagePeople(ctx *gin.Context, memberStore *memebership.Store) {
	roomID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || roomID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	userID := strings.TrimSpace(ctx.Param("userId"))
	if userID == "" || len(userID) > 64 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid target user ID"})
		return
	}

	var req struct {
		Role string `json:"role"`
	}

	if ctx.Request.Method == http.MethodPut {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
	}

	role := strings.TrimSpace(req.Role)
	if ctx.Request.Method == http.MethodDelete {
		role = ""
	}

	if ctx.Request.Method == http.MethodPut && (role != "co_host" && role != "listener") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "role must be co_host or listener"})
		return
	}

	if err := memberStore.ManageMembers(ctx.Request.Context(), roomID, userID, role, ctx.GetString("clerk_user_id")); err != nil {
		membershipError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "membership updated"})
}

func ShowMembersInroom(ctx *gin.Context, memberStore *memebership.Store) {
	roomID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || roomID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	if err := memberStore.RequireMember(ctx.Request.Context(), roomID, ctx.GetString("clerk_user_id")); err != nil {
		membershipError(ctx, err)
		return
	}
	members, err := memberStore.ShowMembershipDetails(ctx.Request.Context(), roomID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ids := make([]string, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.Member_id)
	}
	names := clerkDisplayNames(ctx.Request.Context(), ids)
	for i := range members {
		members[i].Username = names[members[i].Member_id]
		if members[i].Username == "" {
			members[i].Username = "Listener"
		}
	}
	ctx.JSON(http.StatusOK, members)
}

func membershipError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, memebership.ErrForbidden):
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, memebership.ErrNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, memebership.ErrInvalidRole):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, memebership.ErrHostProtected), errors.Is(err, memebership.ErrCoHostLimit):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		log.Printf("membership operation failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "membership operation failed"})
	}
}

// LeaveRoom marks a member offline while preserving membership and role.
// Identity comes from the verified Clerk session.
func LeaveRoom(ctx *gin.Context, store *memebership.Store) {
	roomID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || roomID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}
	userID := ctx.GetString("clerk_user_id")
	if err := store.SetOnline(ctx.Request.Context(), roomID, userID, false); err != nil {
		membershipError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "left room"})
}

// displayName handles optional Clerk profile fields in one place.
func displayName(profile *clerk.User) string {
	if profile == nil {
		return "Listener"
	}
	if profile.Username != nil && strings.TrimSpace(*profile.Username) != "" {
		return strings.TrimSpace(*profile.Username)
	}
	name := ""
	if profile.FirstName != nil {
		name = *profile.FirstName
	}
	if profile.LastName != nil {
		name += " " + *profile.LastName
	}
	if name = strings.TrimSpace(name); name != "" {
		return name
	}
	return "Listener"
}

// clerkDisplayNames batches unique IDs for both room and membership responses.
func clerkDisplayNames(ctx context.Context, ids []string) map[string]string {
	names := make(map[string]string, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			if _, exists := names[id]; !exists {
				names[id] = "Listener"
				unique = append(unique, id)
			}
		}
	}
	for start := 0; start < len(unique); start += 100 {
		end := start + 100
		if end > len(unique) {
			end = len(unique)
		}
		limit := int64(100)
		profiles, err := user.List(ctx, &user.ListParams{UserIDs: unique[start:end], ListParams: clerk.ListParams{Limit: &limit}})
		if err != nil {
			log.Printf("load Clerk profiles: %v", err)
			continue
		}
		for _, profile := range profiles.Users {
			if profile != nil {
				names[profile.ID] = displayName(profile)
			}
		}
	}
	return names
}
