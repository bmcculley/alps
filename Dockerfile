FROM golang:1.16.6

MAINTAINER bmcculley "bmc@docker.e42.xyz"

WORKDIR /alps

COPY . /alps/

ENTRYPOINT ["/alps/cmd/alps/alps"]