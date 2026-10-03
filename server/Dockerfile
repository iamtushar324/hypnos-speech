FROM golang:1.26.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /speechd ./cmd/speechd
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /speechd /speechd
EXPOSE 8680
ENTRYPOINT ["/speechd"]
