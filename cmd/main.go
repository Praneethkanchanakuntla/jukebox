package main

import (
	"log"

	"jukebox/internal/config"

	"jukebox/internal/api"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Print("connecting with DB")

	db, err := config.CreateConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	router := gin.Default()

	api.RegisterRoomEndpoints(router, db)
	router.Run(":3000")
}
