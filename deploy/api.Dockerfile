FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -trimpath -o /campusclaw ./cmd/api
FROM build AS test
RUN go test ./... && go vet ./...
FROM alpine:3.22
RUN addgroup -S campus && adduser -S -G campus campus
COPY --from=build /campusclaw /app/campusclaw
USER campus
ENTRYPOINT ["/app/campusclaw"]
