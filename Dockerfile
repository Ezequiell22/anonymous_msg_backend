FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /app/server /server
ENV ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
