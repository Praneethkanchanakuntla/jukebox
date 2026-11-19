package main

import (
	"fmt"
	"log"

	"jukebox/internal/api"
	"jukebox/internal/config"
	"jukebox/internal/corn"

	"github.com/gin-gonic/gin"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Print("connecting with DB")

	db, err := config.CreateConnection()
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	temporalClient, temporalWorker, err := corn.StartCleanupWorkflow(db)
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	defer temporalWorker.Stop()

	router := gin.Default()

	api.RegisterRoomEndpoints(router, db)
	if err := router.Run(":3000"); err != nil {
		return fmt.Errorf("run HTTP server: %w", err)
	}
	return nil
}
