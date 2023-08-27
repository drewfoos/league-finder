package main

import (
	"fmt"

	"github.com/drewfoos/league-finder/internal/handlers"
	"github.com/drewfoos/league-finder/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func main() {
	handlers.InitApiKey()

	// Create a new Fiber instance
	app := fiber.New()

	// Add middleware
	app.Use(middleware.Cors())

	// Define routes
	app.Post("/search", handlers.SearchHandler)

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("Server is healthy")
	})

	// Start the server
	err := app.Listen(":3000")
	if err != nil {
		fmt.Println(err)
	}
}
