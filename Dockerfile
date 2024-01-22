FROM golang:1.19
WORKDIR /app
COPY go.mod go.sum ./
COPY ./cmd ./cmd
COPY ./pkg ./pkg
RUN go mod vendor
RUN go install github.com/go-bindata/go-bindata/...@latest
RUN go-bindata -pkg bindata -o bindata/bindata.go -prefix "pkg/web/public/" pkg/web/public/...
RUN go build --tags "fts5" -o /dkf ./cmd/dkf/main.go
CMD ["/dkf", "--host=0.0.0.0", "--no-browser"]
EXPOSE 8080