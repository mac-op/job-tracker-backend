package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
)

type JobApplication struct {
	Title       string `json:"title" dynamodbav:"title"`
	Description string `json:"description" dynamodbav:"description"`
	Company     string `json:"company" dynamodbav:"company"`
	Location    string `json:"location" dynamodbav:"location"`
	URL         string `json:"url" dynamodbav:"url"`
	DatePosted  string `json:"date_posted" dynamodbav:"date_posted"`
	InternalId  string `json:"internal_id" dynamodbav:"internal_id"`
	Source      string `json:"source" dynamodbav:"source"`
	Reposted    bool   `json:"reposted" dynamodbav:"reposted"`
	DateApplied string `json:"date_applied" dynamodbav:"date_applied"`
	NumFiles    int    `json:"num_files" dynamodbav:"num_files"`
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

	entry := DBEntry{
		ID:             uuid.NewString(),
		Files:          make([]string, len(r.Files)),
		JobApplication: r.JobApplication,
	}

	for i, file := range r.Files {
		content, _ := readFile(file)
		timestamp := time.Now()
		extension := filepath.Ext(file.Filename)
		fileName := strings.TrimSuffix(file.Filename, extension)
		key := fmt.Sprintf("%s-%02d%02d-%02d%02d%02d_%s%s",
			fileName,
			timestamp.Hour(),
			timestamp.Minute(),
			timestamp.Day(),
			timestamp.Month(),
			timestamp.Year()%100,
			entry.ID,
			extension,
		)

		err := uploadFile(service.S3, *service.S3Bucket, key, content)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to upload file: " + err.Error()})
			return
		}
		entry.Files[i] = key
	}
	e := putApplication(service, entry)
	if e != nil {
		c.JSON(500, gin.H{"error": "Failed to store application: " + e.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Job application received", "application": entry, "num_files": len(r.Files)})
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
