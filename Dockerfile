# Use the offical golang image to create a binary.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:latest as builder

CMD HOME=/root

# Create and change to the app directory.
WORKDIR /bin

# Retrieve application dependencies.
# This allows the container build to reuse cached dependencies.
# Expecting to copy go.mod and if present go.sum.
COPY go.* ./
RUN go mod download

# Copy local code to the container image.
COPY . ./

# Build the binary.
RUN CGO_ENABLED=0 go build -v -o server ./cmd/server/

# Use the official Debian slim image for a lean production container.
# https://hub.docker.com/_/debian
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds
#FROM gcr.io/distroless/static-debian12
FROM scratch

# Copy the binary to the production image from the builder stage.
COPY --from=builder /bin/server/server /app/server
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/


# Run the web service on container startup.
CMD ["/app/server"]

