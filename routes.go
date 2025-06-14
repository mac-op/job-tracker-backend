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
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Company     string    `json:"company"`
	Location    string    `json:"location"`
	URL         string    `json:"url"`
	DatePosted  string    `json:"date_posted"`
	InternalId  string    `json:"internal_id"`
	Source      string    `json:"source"`
	Reposted    bool      `json:"reposted"`
	DateApplied time.Time `json:"date_applied"`
	NumFiles    int       `json:"num_files"`
}

type JobAppRequest struct {
	JobApplication JobApplication          `form:"application" binding:"required"`
	Files          []*multipart.FileHeader `form:"files"`
}

func getFileLink(c *gin.Context) {
	var service *Service
	if v, exists := c.MustGet("services").(*Service); exists {
		service = v
	} else {
		c.JSON(500, gin.H{"error": "Service not initialized"})
		panic("Service not initialized")
		return
	}

	id := c.Param("id")
	fileName := c.Param("file_name")
	if fileName == "" {
		c.JSON(400, gin.H{"error": "File name is required"})
		return
	}
	sep := strings.LastIndex(fileName, "_")
	dot := strings.LastIndex(fileName, ".")
	if id != fileName[sep+1:dot] {
		c.JSON(400, gin.H{"error": "File ID does not match the requested ID"})
		return
	}

	url, err := getFileURL(service, fileName)
	if err != nil {
		fmt.Println("Failed to get file URL: ", err.Error())
		c.JSON(500, gin.H{"error": "Failed to get file URL"})
		return
	}

	c.JSON(200, gin.H{"url": url})
}

func uploadPosting(c *gin.Context) {
	var r JobAppRequest
	if err := c.ShouldBindWith(&r, binding.FormMultipart); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var service *Service
	if v, exists := c.MustGet("services").(*Service); exists {
		service = v
	} else {
		c.JSON(500, gin.H{"error": "Service not initialized"})
		panic("Service not initialized")
		return
	}

	entry := NewDBEntry(uuid.NewString(), make([]string, len(r.Files)), &r.JobApplication)

	for i, file := range r.Files {
		content, _ := readMultipartFile(file)
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
			fmt.Println("Failed to upload file: ", err.Error())
			c.JSON(500, gin.H{"error": "Failed to upload file"})
			return
		}
		entry.Files[i] = key
	}
	e := putApplication(service, &entry)
	if e != nil {
		fmt.Println("Failed to store application: ", e.Error())
		if err := deleteFiles(entry.Files, service.S3, *service.S3Bucket); err != nil {
			fmt.Println("Error deleting files after failed upload:", err)
			c.JSON(
				500,
				gin.H{"error": "Failed to delete uploaded files after failing to store application "},
			)
			return
		}
		c.JSON(500, gin.H{"error": "Failed to store application"})
		return
	}
	c.JSON(200, gin.H{"message": "Job application received", "application": entry, "num_files": len(r.Files)})
}

func readMultipartFile(f *multipart.FileHeader) ([]byte, error) {
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

func updateApp(context *gin.Context) {
	var service *Service
	if v, exists := context.MustGet("services").(*Service); exists {
		service = v
	}
	var app JobApplication
	if err := context.ShouldBindJSON(&app); err != nil {
		fmt.Println("Failed to bind JSON: ", err.Error(), " for data: ", context.Request.Body)
		context.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}
	if e := editApplication(service, context.Param("id"), &app); e != nil {
		fmt.Println("Failed to update application: ", e.Error())
		context.JSON(500, gin.H{"error": "Failed to update application"})
		return
	}
	context.JSON(200, gin.H{"message": "Application updated successfully", "application": app})
}

func getApps(c *gin.Context) {
	var service *Service
	if v, exists := c.MustGet("services").(*Service); exists {
		service = v
	} else {
		c.JSON(500, gin.H{"error": "Service not initialized"})
		panic("Service not initialized")
		return
	}

	var query FilterQuery
	if err := c.ShouldBind(&query); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	apps, err := QueryApplications(service, &query)

	if err != nil {
		fmt.Println("Failed to query applications: ", err.Error())
		c.JSON(500, gin.H{"error": "Failed to query applications"})
		return
	}
	c.JSON(200, gin.H{"results": apps})
}
