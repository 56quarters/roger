FROM alpine:3.12
COPY roger /usr/local/bin/roger
USER nobody
ENTRYPOINT ["roger"]
