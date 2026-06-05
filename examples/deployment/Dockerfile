FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o helix-app ./examples/deployment

FROM gcr.io/distroless/static-debian11
COPY --from=builder /app/helix-app /helix-app
COPY --from=builder /app/examples/deployment/application.yaml /application.yaml
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/helix-app", "--health-check"]
ENTRYPOINT ["/helix-app"]
