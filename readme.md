# DarkForest
Welcome traveler, the goal of this website is to provide a decent chat experience for the good folks using tor without javascript.

Enjoy your stay and spread the word :)
demo :- http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion
 
# How to run

Install go-bindata
```
go install github.com/go-bindata/go-bindata/...@latest
```

Download dependencies
```
go mod vendor
```

## Run at least once

```
make bindata-dev
```

## Serve

```
make serve
```

### Manual run
```
go run --tags "fts5" cmd/dkf/main.go --no-browser
```
