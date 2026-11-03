# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
FROM golang:1.27.3@sha256:2920bb124f1a2e12a1d39f0cab756ae0068441ee1da7655dec7f5e374944095b AS build
WORKDIR /src
COPY go.mod go.sum ./
ENV GOPRIVATE=github.com/symbioticfi/merkle-executor
RUN --mount=type=secret,id=merkle_sdk_read_token,required=true \
    GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0=credential.https://github.com.helper \
    GIT_CONFIG_VALUE_0='!f() { read sdk_token < /run/secrets/merkle_sdk_read_token; printf "username=x-access-token\npassword=%s\n" "$sdk_token"; }; f' \
    go mod download
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
