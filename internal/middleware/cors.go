package middleware

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load(".environment.env")
	if err != nil {
		log.Fatal("Error loading .environment.env file")
	}
}

func Cors() fiber.Handler {
	env := os.Getenv("ENV")
	allowOrigins := "https://riftradar.vercel.app"
	if env == "dev" {
		allowOrigins = "http://localhost:3000"
	}
	return cors.New(cors.Config{
		AllowOrigins: allowOrigins,
		AllowHeaders: "Origin, Content-Type",
		AllowMethods: "POST, OPTIONS",
	})
}
