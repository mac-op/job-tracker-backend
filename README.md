## Job Application Tracker Backend

A Go-based backend service for the Job Application Tracker browser extension. This service provides API endpoints to
store job applications and related files using AWS services (DynamoDB and S3).

## Overview

This is an implementation of the backend server for [Job Application Tracker browser extension](https://github.com/mac-op/app-tracker-ext), providing secure
storage for job applications and their associated files.

## Tech Stack

- Go & Gin
- AWS SDK v2 for:
    - ~~DynamoDB~~ CockroachDB (for job application data)
    - S3 (for file storage)

## API Endpoints

### `POST /upload`

Uploads a job application and optional associated files.

**Headers**
- Content-Type: multipart/form-data

    - Form Fields
        - application (required): JSON-encoded object representing the job application.
        - files (optional): One or more files to be attached (e.g., resume, cover letter).

`application` JSON Schema:

  ```json
  {
      "title": "string",
      "description": "string",
      "company": "string",
      "location": "string",
      "url": "string",
      "date_posted": "string (YYYY-MM-DD)",
      "internal_id": "string",
      "source": "string",
      "reposted": "boolean",
      "date_applied": "string (YYYY-MM-DD)",
      "num_files": 0
  }
  ```
Sample request:
  ```bash
    POST /upload
    Content-Type: multipart/form-data; boundary=----123
    
    ------123
    Content-Disposition: form-data; name="application"
    Content-Type: application/json
    
    {
    "title": "My Application",
    "description": "This is a sample application",
    "company" : "Test Company"
    }
    ------123
    Content-Disposition: form-data; name="files"; filename="text.txt"
    Content-Type: text/plain
    This is the content of the file.
    ------123
  ```
### `POST /applications`
Queries job applications based on provided filters and pagination options. Top-level fields:

`where` (object) - A collection of conditions, ie. `filters` or groups of conditions (`subgroups`). Subgroups could have their own subgroups and filters. \
`sort_by` (string) - Field to sort on (optional).\
`sort_order` (string) - `asc` or `desc` (optional).\
`limit` (number) - Maximum number of records (optional).\
`page` (number) - Page number for pagination (optional).

**Example:**
```json
{
    "where": {
        "filters": [
            {
                "field": "title",      
                "operator": "contains",       
                "value": "engineer"
            }
        ],
        "subgroups": [
            {
                "filters": [],
                "subgroups": [],
                "operator": "or" 
            }
        ],
        "operator": "and"
    },
    "sort_by": "company",
    "sort_order": "asc", 
    "limit": 10,
    "page": 1
}
```

### `PUT /job/:id`: 
Update a specific job application by ID 
```json
{
    "title": "Updated Title",
    "description": "Updated Description",
    "company": "Updated Company",
    "location": "Updated Location",
    "url": "https://updated-url.com",
    "date_posted": "2023-10-01",
    "internal_id": "12345",
    "source": "Updated Source",
    "reposted": false,
    "date_applied": "2023-10-02"
}
```
## Setup

1. Ensure you have Go 1.24 installed
2. Configure AWS credentials in your environment
3. Set up required environment variables or create a `.env` file
4. Run the application:
   ```bash
   go run .
   ```

The server will start on port 8080 by default.

## AWS Requirements

- AWS credentials with permissions for:
    - DynamoDB: CreateTable, DescribeTable, PutItem
    - S3: CreateBucket, PutObject
- DynamoDB table with the schema matching the DBEntry struct
- S3 bucket (will be created automatically if not exists)

