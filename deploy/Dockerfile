# syntax=docker/dockerfile:1
FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY sdk/go.mod sdk/go.sum ./sdk/
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X github.com/symbioticfi/merkle-executor/internal/version.Version=${VERSION} -X github.com/symbioticfi/merkle-executor/internal/version.Commit=${COMMIT} -X github.com/symbioticfi/merkle-executor/internal/version.Date=${DATE}" \
    -o /merkle-executor ./cmd/merkle-executor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /merkle-executor /merkle-executor
USER nonroot:nonroot
ENTRYPOINT ["/merkle-executor"]
