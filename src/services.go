package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	_ "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/joho/godotenv"
	"os"
	"time"
)

var bucketName string = ""
var Config map[string]string

type Service struct {
	S3       *s3.Client
	S3Bucket *string
}

func initConfig() {
	if Config == nil {
		var err error
		if _, err := os.Stat("../.env"); err != nil {
			if os.IsNotExist(err) {
				return
			}
		}
		Config, err = godotenv.Read("../.env")
		if err != nil {
			panic(err)
		}
	}
}

func GetEnv(key string) string {
	initConfig()
	return Config[key]
}

func InitAWS() (*Service, error) {
	ctx := context.Background()
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	s3Client := s3.NewFromConfig(sdkConfig)
	if e := GetOrCreateBucket(s3Client); e != nil {
		fmt.Println("Error getting or creating bucket:", e)
		return nil, e
	}
	return &Service{
		S3:       s3Client,
		S3Bucket: &bucketName,
	}, nil
}

func uploadFile(c *s3.Client, bucketName string, key string, body []byte) error {
	_, err := c.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &bucketName,
		Key:    &key,
		Body:   bytes.NewReader(body),
	})
	if err != nil {
		fmt.Println("Error uploading file:", err)
		return err
	}
	return nil
}

func GetOrCreateBucket(client *s3.Client) error {
	if bucketName == "" {
		bucketName = GetEnv("BUCKET_NAME")
	}
	var defaultBucketName = fmt.Sprintf("job-applications-bucket-%d", time.Now().Unix())

	if bucketName == "" {
		region := client.Options().Region
		out, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{
			Bucket: &defaultBucketName,
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(region),
			},
		})
		if err != nil {
			fmt.Println("Error creating bucket:", err)
			return err
		}
		bucketName = *out.Location
	}
	return nil
}
