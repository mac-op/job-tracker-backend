package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
)

type JobApplication struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	DatePosted  string `json:"date_posted"`
	InternalId  string `json:"internal_id"`
	Source      string `json:"source"`
	Reposted    bool   `json:"reposted"`
	DateApplied string `json:"date_applied"`
	NumFiles    int    `json:"num_files"`
}

type JobAppRequest struct {
	JobApplication JobApplication          `form:"application" binding:"required"`
	Files          []*multipart.FileHeader `form:"files"`
}

func uploadPosting(c *gin.Context) {
	var r JobAppRequest
	if err := c.ShouldBindWith(&r, binding.FormMultipart); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var service *Service
	if v, exists := c.MustGet("aws").(*Service); exists {
		service = v
	} else {
		c.JSON(500, gin.H{"error": "Service not initialized"})
		panic("Service not initialized")
		return
	}

	for _, file := range r.Files {
		content, _ := readFile(file)
		timestamp := time.Now()
		extension := filepath.Ext(file.Filename)
		fileName := strings.TrimSuffix(file.Filename, extension)
		key := fmt.Sprintf("%s-%02d%02d-%02d%02d%02d%s",
			fileName,
			timestamp.Hour(),
			timestamp.Minute(),
			timestamp.Day(),
			timestamp.Month(),
			timestamp.Year()%100,
			extension,
		)

		err := uploadFile(service.S3, *service.S3Bucket, key, content)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to upload file: " + err.Error()})
			return
		}
	}
	c.JSON(200, gin.H{"message": "Job application received", "application": r.JobApplication, "num_files": len(r.Files)})
}

func readFile(f *multipart.FileHeader) ([]byte, error) {
	content, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer content.Close()

	buf := make([]byte, f.Size)
	_, err = content.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
