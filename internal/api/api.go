package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jukebox/internal/room"

	"github.com/gin-gonic/gin"
)

func RegisterRoomEndpoints(route *gin.Engine, db *sql.DB) {
	store := room.NewStore(db)
	roomGroup := route.Group("/room")
	roomGroup.GET("/openRoom/:id", func(ctx *gin.Context) {
		OpenRoom(ctx, store)
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
}

func CreateRooms(ctx *gin.Context, store *room.Store) {
	var roomReq room.CreateRoom
	if err := ctx.ShouldBind(&roomReq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	roomId, err := store.CreateRoom(ctx.Request.Context(), roomReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusAccepted, gin.H{
		"id":   roomId,
		"name": roomReq.Name,
	})
	return
}

func GetRooms(ctx *gin.Context, store *room.Store) {
	rooms, err := store.GetAllRooms(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		log.Fatal("error while getting rooms", err)
	}
	ctx.JSON(http.StatusOK, rooms)
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

	rows, err := store.UpdateRoom(c.Request.Context(), id, room.UpdateRoom{
		Name:   req.Name,
		Locked: req.Locked,
	})
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
func OpenRoom(ctx *gin.Context, store *room.Store) *room.Room {
	roomParam := ctx.Param("id")
	roomId, err := strconv.Atoi(roomParam)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid room id"})
		return nil
	}

	passCode := ctx.Query("passCode")

	dbRoom, err := store.GetByID(ctx.Request.Context(), int64(roomId))
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

	ctx.JSON(200, dbRoom)
	return dbRoom
}
