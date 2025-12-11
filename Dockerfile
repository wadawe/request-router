ARG ARCH=arm64
ARG GO_VERSION=1.24.2

FROM --platform=linux/${ARCH} rockylinux:9

# Update system packages
RUN yum update -y
RUN yum install -y wget tar git
RUN yum clean all
RUN rm -rf /var/cache/yum

# Install go
ARG ARCH GO_VERSION
ENV ARCH=${ARCH} GO_VERSION=${GO_VERSION}
WORKDIR /go/src/github.com/wadawe/request-router
RUN wget "https://dl.google.com/go/go${GO_VERSION}.linux-${ARCH}.tar.gz"
RUN tar -C /usr/local -xzf "go${GO_VERSION}.linux-${ARCH}.tar.gz"
ENV PATH="/usr/local/go/bin:${PATH}"
RUN chmod -R 777 /usr/local/go/
RUN chmod -R 777 "/go"

# Copy build source
COPY go.* ./
COPY pkg/ ./pkg/
COPY build.go ./
COPY CHANGELOG.md ./
COPY .git/ ./.git/

# Build the source once
# =========
# Run with: `$ make all`
# =========
CMD [ "go", "run", "build.go", "build" ]

### OR ###

# Build the source & long run for debugging
# =========
# Run with: `$ docker compose up --build --force-recreate -d`
# Dev with: `$ docker exec -it <container_id> /bin/bash`
# End with: `$ docker compose down`
# =========
# RUN go run build.go build > /var/log/build.log 2>&1
# ENTRYPOINT [ "tail", "-f", "/dev/null" ]
