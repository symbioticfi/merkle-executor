# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
FROM golang:1.27.3-alpine3.24@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS build
RUN apk add --no-cache git
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

FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99
COPY --from=build /merkle-executor /merkle-executor
USER nonroot:nonroot
ENTRYPOINT ["/merkle-executor"]
