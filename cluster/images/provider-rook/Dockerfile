FROM BASEIMAGE
RUN apk --no-cache add ca-certificates bash

ARG ARCH
ARG TINI_VERSION

ADD stack /usr/local/bin/crossplane-rook-stack

COPY package /

EXPOSE 8080
USER 1001
ENTRYPOINT ["crossplane-rook-stack"]
