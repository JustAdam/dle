FROM busybox:ubuntu-14.04

MAINTAINER JustAdam <adambell7@gmail.com>

ADD dle /usr/bin/dle
ADD certs.pem /etc/dle/certs.pem

WORKDIR /etc/dle/

ENTRYPOINT ["/usr/bin/dle"]