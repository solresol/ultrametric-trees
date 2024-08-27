.PHONY: build run test clean dbclean

build: bin/prepare bin/train
	echo All built

bin/train: cmd/train/main.go pkg/exemplar/exemplar.go
	go build -o bin/train cmd/train/main.go

bin/prepare: cmd/prepare/main.go
	go build -o bin/prepare cmd/prepare/main.go

slm-w2.sqlite: bin/prepare /tinystories/wordnetify-tinystories/w2.sqlite
	./bin/prepare --input-database /tinystories/wordnetify-tinystories/w2.sqlite --output-database slm-w2.sqlite

test:
	go test ./...

clean:
	rm -rf bin/prepare bin/exemplar

dbclean:
	rm -f slm-w2.sqlite

