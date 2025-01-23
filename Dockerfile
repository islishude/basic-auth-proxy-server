# syntax=docker/dockerfile:1
FROM golang:1.23.5 AS compiler
WORKDIR /app
COPY . ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go install -v .

FROM gcr.io/distroless/base-debian12:latest
COPY --from=compiler /go/bin/* /usr/local/bin/
ENTRYPOINT ["basic-auth-proxy-server"]
