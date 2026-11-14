# Build stage: compile both binaries with no CGO for a static image.
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/qmart-api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/qmart-seed ./cmd/seed

# Runtime stage: minimal, non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/qmart-api /usr/local/bin/qmart-api
COPY --from=build /out/qmart-seed /usr/local/bin/qmart-seed

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/qmart-api"]
