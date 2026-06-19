FROM alpine/git

RUN apk add --no-cache curl aws-cli

COPY localstack-init.sh /init/localstack-init.sh
COPY ogit-init.sh /init/ogit-init.sh
COPY default-features.yaml /init/default-features.yaml

RUN chmod +x /init/*.sh
