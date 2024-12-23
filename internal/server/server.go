package server

import (
	"github.com/gofiber/fiber/v2"

	"rytr/internal/database"
)

type FiberServer struct {
	*fiber.App

	db database.Service
}

func New() *FiberServer {
	server := &FiberServer{
		App: fiber.New(fiber.Config{
			ServerHeader: "rytr",
			AppName:      "rytr",
		}),

		db: database.New(),
	}

	return server
}
