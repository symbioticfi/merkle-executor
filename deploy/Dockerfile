# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
FROM golang:1.27.4-alpine3.24@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS build
RUN apk add --no-cache git
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

FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
COPY --from=build /merkle-executor /merkle-executor
USER nonroot:nonroot
ENTRYPOINT ["/merkle-executor"]
