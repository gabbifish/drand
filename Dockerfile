FROM golang:1.12.0@sha256:99ea21f08666ff99def73294c2f85a3869b5fcacc2535ba51e315766a25b9626 as builder

ARG SRC_ROOT=/usr/local/go/src/code.cfops.it/crypto/drand

COPY . $SRC_ROOT/
WORKDIR $SRC_ROOT

RUN go install -mod=vendor && rm -rf "/go/src/code.cfops.it/crypto/drand"

FROM docker-registry.cfdata.org/stash/plat/dockerfiles/debian-stretch/master:2019.1.0

RUN apt-get update && \
  apt-get install -y build-essential prometheus-node-exporter s3cmd && \
  mkdir -p /root/.drand/groups && mkdir -p /root/.drand/key && \
  chmod 740 /root/.drand

COPY ./drand.s3cfg            /root/.s3cfg
COPY ./scripts/prepare.sh     /prepare.sh
COPY ./scripts/end.sh         /end.sh
COPY --from=builder go/bin/drand /drand

RUN chmod 755 /drand
RUN chmod 755 /prepare.sh
RUN chmod 755 /end.sh

