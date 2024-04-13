# syntax=docker/dockerfile:1
FROM golang:1.22.2 as BUILDER
WORKDIR /app
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v .

FROM gcr.io/distroless/base-debian12:latest as latest
COPY --from=BUILDER /go/bin/* /usr/local/bin/
ENTRYPOINT ["basic-auth-proxy-server"]
