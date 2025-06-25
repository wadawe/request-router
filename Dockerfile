# FROM --platform=linux/amd64 rockylinux:8
FROM --platform=linux/arm64 rockylinux:8

# Install wget
RUN yum install -y wget
RUN yum install -y git

# Install go
ENV GO_VERSION=1.21.3
WORKDIR /go/src/github.com/wadawe/request-router
# RUN wget "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz"
# RUN tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
RUN wget "https://dl.google.com/go/go${GO_VERSION}.linux-arm64.tar.gz"
RUN tar -C /usr/local -xzf "go${GO_VERSION}.linux-arm64.tar.gz"
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
# Run with: `$ docker compose up --build --force-recreate`
# End with: `$ docker compose down`
# =========
CMD [ "go", "run", "build.go", "build" ]

# OR

# Build the source & long run for debugging
# =========
# Run with: `$ docker compose up --build --force-recreate -d`
# End with: `$ docker compose down`
# =========
# RUN go run build.go build > /var/log/build.log 2>&1
# ENTRYPOINT [ "tail", "-f", "/dev/null" ]
