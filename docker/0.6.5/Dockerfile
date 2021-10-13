FROM alpine:3.14

RUN set -eux; \
	adduser -u 1000 -H -h /opt -D -s /bin/sh api-firewall

ENV APIFIREWALL_VERSION 0.6.5
ENV APIFIREWALL_URL https://github.com/wallarm/api-firewall/releases/download/v0.6.5/api-firewall_linux-amd64
ENV APIFIREWALL_SHA256 599f3e3bb385742cc978da85d359c81bfc02d8d1a9cacef551b9bfe8b3a4603a

RUN set -eux; \	
	\
	wget -O api-firewall "$APIFIREWALL_URL"; \
	echo "$APIFIREWALL_SHA256 *api-firewall" | sha256sum -c; \
	\
	mv api-firewall /usr/local/bin/api-firewall; \
	chmod +x /usr/local/bin/api-firewall; \
	\
# smoke test
	api-firewall -v

COPY docker-entrypoint.sh /usr/local/bin/
ENTRYPOINT ["docker-entrypoint.sh"]

USER api-firewall
CMD ["api-firewall"]
