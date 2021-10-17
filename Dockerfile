FROM alpine:3.14
COPY roger /usr/local/bin/roger
USER nobody
ENTRYPOINT ["roger"]
