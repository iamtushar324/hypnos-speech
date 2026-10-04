FROM golang:1.26.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /vokirid ./cmd/vokirid
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /vokirid /vokirid
EXPOSE 8680
ENTRYPOINT ["/vokirid"]
