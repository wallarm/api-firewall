FROM python:3.8.0

WORKDIR /tmp

COPY docs/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Set working directory
WORKDIR /docs
VOLUME /docs
RUN rm -rf docs

EXPOSE 8000
ENTRYPOINT ["mkdocs"]
CMD ["serve", "--dev-addr=0.0.0.0:8000", "--config-file=mkdocs.yml"]

# docker run --rm -it -p 8000:8000/tcp -v ${PWD}:/docs docs-apifirewall:latest