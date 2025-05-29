FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o main .

FROM public.ecr.aws/lambda/provided:al2

COPY --from=builder /app/main /var/task/

COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:0.7.0 /lambda-adapter /opt/extensions/lambda-adapter

COPY .env /var/task/

ENV PORT=8080
ENV AWS_LIE=true

ENTRYPOINT [ "/var/task/main" ]