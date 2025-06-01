package main

import (
	"github.com/gin-gonic/gin"
)

func setUp(s *Service) *gin.Engine {
	engine := gin.Default()
	engine.Use(gin.Logger())

	engine.Use(func(c *gin.Context) {
		c.Set("services", s)
		c.Next()
	})

	engine.POST("/upload", uploadPosting)
	engine.POST("/applications", getApps)

	return engine
}

func main() {
	s, err := InitService()
	if err != nil {
		panic("Failed to initialize service: " + err.Error())
	}
	engine := setUp(s)
	if err := engine.Run(":8080"); err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
