# Run with docker

```
docker build -t dkf .
docker run -p 8080:8080 -v /path/to/your/.dkf:/root/.dkf --name dkf dkf
```

# How to run

Install go-bindata
```
go install github.com/go-bindata/go-bindata/...@latest
```

Install "air" (for live-reload development)
```
go install github.com/cosmtrek/air@latest
```

Download dependencies
```
go mod vendor
```

## Run at least once

```
make bindata-dev
```

## Build qtpl templates

```
go install github.com/valyala/quicktemplate/qtc@latest
```

## Serve

```
make serve
```

### Manual run
```
go run --tags "fts5" cmd/dkf/main.go --no-browser
```

### Build prohibited passwords list from rockyou.txt

Download rockyou.txt

```
curl -L https://github.com/brannondorsey/naive-hashcat/releases/download/data/rockyou.txt -o rockyou.txt
```

Import rockyou.txt in database

```
./dkf build-prohibited-passwords
```


# Notes for running darkforest on a server

```
useradd --system --create-home dkf
```