package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

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
}

func CreateRooms(ctx *gin.Context, store *room.Store) {
	var roomReq room.CreateRoom
	if err := ctx.ShouldBind(&roomReq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	roomId, err := store.CreateRoom(ctx, roomReq)
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
	rooms, err := store.GetAllRooms(ctx)
	if err != nil {
		log.Fatal("error while getting rooms", err)
	}
	ctx.JSON(http.StatusOK, rooms)
}

func OpenRoom(ctx *gin.Context, store *room.Store) *room.Room {
	roomParam := ctx.Param("id")
	roomId, err := strconv.Atoi(roomParam)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "invalid room id"})
		return nil
	}

	passCode := ctx.Query("passCode")

	dbRoom, err := store.GetByID(ctx, int64(roomId))
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
