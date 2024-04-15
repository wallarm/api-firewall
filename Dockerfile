FROM golang:1.21-alpine3.18 AS build

ARG APIFIREWALL_VERSION
ENV APIFIREWALL_VERSION=${APIFIREWALL_VERSION}

RUN apk add --no-cache                       \
        gcc                                  \
        git                                  \
        make                                 \
        musl-dev

WORKDIR /build
COPY . .

RUN go mod download -x                    && \
    go build                                 \
        -ldflags="-X main.build=${APIFIREWALL_VERSION} -linkmode 'external' -extldflags '-static' -s -w" \
        -buildvcs=false                      \
        -o ./api-firewall                    \
        ./cmd/api-firewall

# Smoke test
RUN ./api-firewall -v

FROM alpine:3.18 AS composer

WORKDIR /output

COPY --from=build /build/api-firewall ./usr/local/bin/
COPY docker-entrypoint.sh ./usr/local/bin/docker-entrypoint.sh

RUN chmod 755 ./usr/local/bin/*           && \
    chown root:root ./usr/local/bin/*

FROM alpine:3.18

RUN adduser -u 1000 -H -h /opt -D -s /bin/sh api-firewall

COPY --from=composer /output /

USER api-firewall
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["api-firewall"]
