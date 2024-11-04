# Stage 1: Build the Go application
FROM golang:1.23-alpine AS build

# Install necessary packages
RUN apk --no-cache add git

# Set environment variables for Go
ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the Go application
RUN go build -o main .

# Stage 2: Create a minimal Docker image to run the application
FROM alpine:latest

# Set the working directory in the final image
WORKDIR /root/

# Copy the compiled Go binary from the build stage
COPY --from=build /app/main .

# Set environment variables for Fiber
ENV PORT=3000

# Expose the port the application runs on
EXPOSE 3000

# Run the Go application
CMD ["./main"]
