# ../Dockerfile
FROM golang:1.25-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download 


COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=build /app/api /app/api
COPY --from=build /app/configs /app/configs/

EXPOSE 8080
ENTRYPOINT [ "/app/api" ]