package server

import (
	"context"
	"log"
	"os"
	"rytr/internal/database"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"google.golang.org/genai"
)

type FiberServer struct {
	*fiber.App

	db           database.Service
	geminiClient *genai.Client
}

func New() *FiberServer {
	server := &FiberServer{
		App: fiber.New(fiber.Config{
			ServerHeader: "rytr",
			AppName:      "rytr",
		}),
		db: database.New(),
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	server.geminiClient = client
	server.App.Use(favicon.New())
	server.App.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173, https://rytr.fuzzydevs.com", // Your React app's URL
		AllowHeaders: "Origin, Content-Type, Accept, Authorization,X-Requested-With",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		// Optional: Enable preflight request caching
		MaxAge: 3600,
	}))
	server.App.Use(logger.New())
	server.App.Use(pprof.New(pprof.Config{
		Next: nil, // Use this if you want to exclude specific routes
	}))
	return server
}
