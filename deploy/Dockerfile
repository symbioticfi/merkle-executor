# syntax=docker/dockerfile:1
FROM golang:1.27.1 AS build
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

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /merkle-executor /merkle-executor
USER nonroot:nonroot
ENTRYPOINT ["/merkle-executor"]
