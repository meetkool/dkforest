# Use an official Golang runtime as the base image
FROM golang:1.19 alpine as builder

# Set the working directory to /app
WORKDIR /app

# Copy the go.mod and go.sum files to the working directory
COPY go.mod go.sum ./

# Copy the local package directory to the builder stage's working directory
COPY ./pkg ./pkg

# Copy the local cmd directory to the builder stage's working directory
COPY ./cmd ./cmd

# Download and install go-bindata
RUN apk add --no-cache git && \
    go get -d github.com/go-bindata/go-bindata/... && \
    go install github.com/go-bindata/go-bindata/...

# Run go mod vendor to get all dependencies
RUN go mod vendor

# Use go-bindata to generate bindata.go from the pkg/web/public directory
RUN go-bindata -pkg bindata -o bindata/bindata.go -prefix "pkg/web/public/" pkg/web/public/...

# Build the binary using the vendor directory and the fts5 tag
RUN CGO_ENABLED=0 GOOS=linux go build --tags "fts5" -o /dkf ./cmd/dkf/main.go

# Use an Alpine Linux image as the final stage
FROM alpine:latest

# Copy the binary from the builder stage to the final stage
COPY --from=builder /dkf /dkf

# Set the working directory to /app
WORKDIR /app

# Expose port 8080
EXPOSE 8080

# Run the dkf binary with the --host=0.0.0.0 and --no-browser flags
CMD ["/dkf", "--host=0.0.0.0", "--no-browser"]
