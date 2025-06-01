package main

import (
	"bytes"
	"context"
	_ "database/sql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"strconv"
	"strings"

	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"log"

	_ "errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"os"
	"time"
)

var bucketName string = ""

type Service struct {
	S3       *s3.Client
	S3Bucket *string
	db       *pgxpool.Pool
}

type DBEntry struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Company     string             `json:"company"`
	Description string             `json:"description"`
	Location    string             `json:"location"`
	DatePosted  string             `json:"date_posted"`
	URL         string             `json:"url"`
	InternalId  string             `json:"internal_id"`
	Source      string             `json:"source"`
	Reposted    bool               `json:"reposted"`
	DateApplied pgtype.Timestamptz `json:"date_applied"`
	Files       []string           `json:"files"`
}

func NewDBEntry(id string, files []string, jobApp *JobApplication) DBEntry {
	return DBEntry{
		ID:          id,
		Title:       jobApp.Title,
		Company:     jobApp.Company,
		Description: jobApp.Description,
		Location:    jobApp.Location,
		DatePosted:  jobApp.DatePosted,
		URL:         jobApp.URL,
		InternalId:  jobApp.InternalId,
		Source:      jobApp.Source,
		Reposted:    jobApp.Reposted,
		DateApplied: pgtype.Timestamptz{Time: jobApp.DateApplied, Valid: true},
		Files:       files,
	}
}

const (
	and                   = "AND"
	or                    = "OR"
	equals                = "="
	like                  = "LIKE"
	contains              = "CONTAINS"
	not_equals            = "!="
	less_than             = "<"
	greater_than          = ">"
	less_than_or_equal    = "<="
	greater_than_or_equal = ">="
)

var numericOps = []string{equals, not_equals, less_than, greater_than, less_than_or_equal, greater_than_or_equal}

type Filter struct {
	Operator string `json:"operator" binding:"required"`
	Lhs      string `json:"field" binding:"required"`
	Rhs      string `json:"value" binding:"required"`
}

func (f *Filter) BuildFilter() string {
	if f == nil || f.Lhs == "" || f.Operator == "" || f.Rhs == "" {
		return ""
	}
	if strings.ToUpper(f.Operator) == contains {
		return fmt.Sprintf("%s LIKE '%%%s%%'", f.Lhs, f.Rhs)
	}

	for _, op := range numericOps {
		if f.Operator != op {
			continue
		}
		if _, e := strconv.Atoi(f.Rhs); e != nil {
			return fmt.Sprintf("%s %s '%s'", f.Lhs, f.Operator, f.Rhs)
		} else {
			return fmt.Sprintf("%s %s %s", f.Lhs, f.Operator, f.Rhs)
		}
	}
	return fmt.Sprintf("%s %s '%s'", f.Lhs, f.Operator, f.Rhs)
}

type FilterGroup struct {
	Filters   []Filter       `json:"filters"`
	SubGroups []*FilterGroup `json:"subgroups"`
	Operator  string         `json:"operator"` // Only "AND" or "OR" are allowed
}

func (g *FilterGroup) BuildGroup() string {
	if g == nil || len(g.Filters) == 0 {
		return ""
	}

	var result string
	for i, filter := range g.Filters {
		if i > 0 && i < len(g.Filters) {
			result += fmt.Sprintf(" %s ", g.Operator)
		}
		result += filter.BuildFilter()
	}

	//if len(g.Filters) > 0 && len(g.SubGroups) > 0 {
	//	result += fmt.Sprintf(" %s ", g.Operator)
	//}

	for _, subgroup := range g.SubGroups {
		subResult := subgroup.BuildGroup()
		if subResult != "" {
			if result != "" {
				result += fmt.Sprintf(" %s ", g.Operator)
			}
			result += fmt.Sprintf("(%s)", subResult)
		}
	}

	return result
}

type FilterQuery struct {
	FilterGroup *FilterGroup `json:"where"`
	SortBy      string       `json:"sort_by"`
	SortOrder   string       `json:"sort_order"`
	Limit       int          `json:"limit"`
}

func (fq *FilterQuery) BuildQuery() string {
	result := `SELECT 
    	id,
    	title,
    	company,
    	description,
    	location,
    	date_posted,
    	url,
    	internal_id,
    	source,
    	reposted,
    	date_applied,
    	files
    FROM job_application `

	if fq.FilterGroup != nil {
		filterGroup := fq.FilterGroup.BuildGroup()
		if filterGroup != "" {
			result += "WHERE " + filterGroup + " "
		}
	}
	if fq.SortBy != "" {
		order := "ASC"
		if strings.ToLower(fq.SortOrder) == "desc" {
			order = "DESC"
		}
		result += fmt.Sprintf("ORDER BY %s %s ", fq.SortBy, order)
	}
	if fq.Limit > 0 {
		result += fmt.Sprintf("LIMIT %d", fq.Limit)
	}

	return result
}

type QueryBuilder struct {
	query *FilterQuery
	conn  *pgxpool.Pool
}

func (q *QueryBuilder) Execute() ([]DBEntry, error) {
	if q.query == nil || q.conn == nil {
		return nil, fmt.Errorf("query or connection not initialized")
	}

	sqlQuery := q.query.BuildQuery()
	if sqlQuery == "" {
		return nil, fmt.Errorf("invalid query: no filters or conditions specified")
	}
	fmt.Println("Executing query:", sqlQuery)
	var results []DBEntry

	err := crdbpgx.ExecuteTx(context.Background(), q.conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), sqlQuery)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			e := DBEntry{}
			if sErr := rows.Scan(
				&e.ID, &e.Title, &e.Company, &e.Description, &e.Location, &e.DatePosted,
				&e.URL, &e.InternalId, &e.Source, &e.Reposted, &e.DateApplied, &e.Files); sErr != nil {
				return sErr
			}
			results = append(results, e)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func QueryApplications(s *Service, query *FilterQuery) ([]DBEntry, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("service or database not initialized")
	}

	builder := &QueryBuilder{
		query: query,
		conn:  s.db,
	}
	return builder.Execute()
}

func InitService() (*Service, error) {
	s, err := InitAWS()
	if err != nil {
		fmt.Println("Error initializing AWS:", err)
		return nil, err
	}
	s.db = InitDB()
	if s.db == nil {
		fmt.Println("Error initializing database")
		return nil, fmt.Errorf("failed to initialize database")
	}
	return s, nil
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

	return &Service{
		S3:       s3Client,
		S3Bucket: &bucketName,
	}, nil
}

func InitDB() *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	return pool
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

func putApplication(s *Service, e *DBEntry) error {
	insertFunc := func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO job_application 
    				(id, title, company, description, location, date_posted, url, internal_id, source, reposted, date_applied, files) 
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			e.ID, e.Title, e.Company, e.Description, e.Location, e.DatePosted, e.URL, e.InternalId, e.Source,
			e.Reposted, e.DateApplied, e.Files)
		if err != nil {
			fmt.Println("Error inserting application:", err)
			return err
		}
		return nil
	}
	err := crdbpgx.ExecuteTx(context.Background(), s.db, pgx.TxOptions{}, insertFunc)
	if err != nil {
		return err
	}
	return nil
}

func deleteFiles(files []string, s3Client *s3.Client, bucketName string) error {
	for _, file := range files {
		_, err := s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: &bucketName,
			Key:    &file,
		})
		if err != nil {
			fmt.Println("Error deleting file:", err)
			return err
		}
	}
	return nil
}

func GetOrCreateBucket(client *s3.Client) error {
	if bucketName == "" {
		bucketName = os.Getenv("BUCKET_NAME")
	}
	var defaultBucketName = fmt.Sprintf("job-applications-bucket-%d", time.Now().Unix())

	if bucketName == "" {
		region := client.Options().Region
		_, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{
			Bucket: &defaultBucketName,
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(region),
			},
		})
		if err != nil {
			fmt.Println("Error creating bucket:", err)
			return err
		}
		bucketName = defaultBucketName
	}
	return nil
}
