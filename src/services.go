package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	_ "github.com/aws/aws-sdk-go/service/dynamodb"

	"github.com/joho/godotenv"
	"os"
	"time"
)

var bucketName string = ""
var Config map[string]string = nil

type Service struct {
	S3        *s3.Client
	S3Bucket  *string
	DB        *dynamodb.Client
	tableName string
}

type DBEntry struct {
	ID    string   `dynamodbav:"id"`
	Files []string `dynamodbav:"files"`
	JobApplication
}

func initConfig() {
	if Config == nil {
		var err error
		if _, err = os.Stat("../.env"); err != nil {
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
	result := Config[key]
	if result == "" {
		return os.Getenv(key)
	}
	return result
}

func InitAWS() (*Service, error) {
	ctx := context.Background()
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// S3 Config
	s3Client := s3.NewFromConfig(sdkConfig)
	if e := GetOrCreateBucket(s3Client); e != nil {
		fmt.Println("Error getting or creating bucket:", e)
		return nil, e
	}

	// DynamoDB Config
	dbClient := dynamodb.NewFromConfig(sdkConfig)
	tableName := GetEnv("TABLE_NAME")
	if tableName != "" {
		tableName = "job_application"
	}
	exists, err := checkTableExists(dbClient, tableName)
	if err != nil {
		fmt.Println("Error checking table existence:", err)
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}

	return &Service{
		S3:        s3Client,
		S3Bucket:  &bucketName,
		DB:        dbClient,
		tableName: tableName,
	}, nil
}

func checkTableExists(c *dynamodb.Client, tableName string) (bool, error) {
	_, err := c.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: &tableName,
	})
	if err != nil {
		var notFound *dynamodbtypes.ResourceNotFoundException
		if ok := errors.As(err, &notFound); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

func putApplication(s *Service, item DBEntry) error {
	m, e := attributevalue.MarshalMap(item)
	if e != nil {
		fmt.Println("Error marshalling item:", e)
		return e
	}

	_, err := s.DB.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item:      m,
	})
	if err != nil {
		fmt.Println("Error putting item:", err)
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
			CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(region),
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
