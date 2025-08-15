FROM golang:1.24-alpine3.22 AS build

ARG APIFIREWALL_NAMESPACE
ARG APIFIREWALL_VERSION
ENV APIFIREWALL_NAMESPACE=${APIFIREWALL_NAMESPACE}
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
        -ldflags="-X ${APIFIREWALL_NAMESPACE}/internal/version.Version=${APIFIREWALL_VERSION} -s -w" \
        -buildvcs=false                      \
        -o ./api-firewall                    \
        ./cmd/api-firewall

# Smoke test
RUN ./api-firewall -v

FROM alpine:3.22 AS composer

WORKDIR /output

COPY --from=build /build/api-firewall ./usr/local/bin/
COPY docker-entrypoint.sh ./usr/local/bin/docker-entrypoint.sh

RUN chmod 755 ./usr/local/bin/*           && \
    chown root:root ./usr/local/bin/*

FROM alpine:3.22

RUN adduser -u 1000 -H -h /opt -D -s /bin/sh api-firewall

COPY --from=composer /output /

USER api-firewall
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["api-firewall"]
