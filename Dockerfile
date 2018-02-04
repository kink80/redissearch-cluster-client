FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/redissearch-cluster-client

# Build redissearch-cluster-client project
RUN go get github.com/kink80/redisearch-go/redisearch && \
    go get github.com/gorilla/mux && \
    go get github.com/hashicorp/consul/api && \
    go get github.com/hashicorp/go-cleanhttp && \
    go install redissearch-cluster-client

ENTRYPOINT /go/bin/redissearch-cluster-client

EXPOSE 5555
