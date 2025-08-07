FROM golang:1.22 AS build
WORKDIR /app
COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o /spyder ./cmd/spyder

FROM gcr.io/distroless/base-debian12
USER nonroot:nonroot
COPY --from=build /spyder /usr/local/bin/spyder

LABEL org.opencontainers.image.source=https://github.com/gustycube/spyder
LABEL org.opencontainers.image.description="SPYDER - System for Probing and Yielding DNS-based Entity Relations"
LABEL org.opencontainers.image.licenses=MIT

ENTRYPOINT ["/usr/local/bin/spyder"]