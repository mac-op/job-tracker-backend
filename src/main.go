package main

import (
	"github.com/gin-gonic/gin"
)

func setUp(s *Service) *gin.Engine {
	engine := gin.Default()
	engine.Use(gin.Logger())

	engine.Use(func(c *gin.Context) {
		c.Set("aws", s)
		c.Next()
	})

	engine.POST("/upload", uploadPosting)

	return engine
}

func main() {
	awsService, err := InitAWS()
	if err != nil {
		panic("Failed to initialize AWS service: " + err.Error())
	}
	engine := setUp(awsService)
	if err := engine.Run(":8080"); err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
