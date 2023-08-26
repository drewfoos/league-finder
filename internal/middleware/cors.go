package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func Cors() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins: "https://riftradar.vercel.app",
		AllowHeaders: "Origin, Content-Type",
		AllowMethods: "POST, OPTIONS",
	})
}
