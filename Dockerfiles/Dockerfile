FROM ubuntu:14.04

MAINTAINER JustAdam <adambell7@gmail.com>

RUN apt-get update && \
    apt-get upgrade -y

ADD dle /usr/bin/dle

VOLUME ["/docker-containers", "/etc/dle/"]

WORKDIR /etc/dle/

CMD ["/usr/bin/dle"]