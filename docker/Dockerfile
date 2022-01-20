FROM golang:1.17-alpine3.15 AS build

ARG APIFIREWALL_VERSION
ENV APIFIREWALL_VERSION=${APIFIREWALL_VERSION}

RUN apk add --no-cache                       \
        git                                  \
        gcc                                  \
        make

WORKDIR /build
COPY . .

RUN go mod download -x                    && \
    go build                                 \
        -ldflags="-X main.build=${APIFIREWALL_VERSION} -s -w" \
        -o ./api-firewall                    \
        ./cmd/api-firewall

# Smoke test
RUN ./api-firewall -v

FROM alpine:3.15 AS composer

WORKDIR /output

COPY --from=build /build/api-firewall ./usr/local/bin/
COPY ./docker/docker-entrypoint.sh ./usr/local/bin/docker-entrypoint.sh

RUN chmod 755 ./usr/local/bin/*           && \
    chown root:root ./usr/local/bin/*

FROM alpine:3.15

RUN adduser -u 1000 -H -h /opt -D -s /bin/sh api-firewall

COPY --from=composer /output /

USER api-firewall
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["api-firewall"]
