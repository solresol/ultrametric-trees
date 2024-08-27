.PHONY: build run test clean dbclean

build: bin/prepare bin/exemplar bin/step
	echo All built

bin/prepare: cmd/prepare/main.go
	go build -o bin/prepare cmd/prepare/main.go

bin/exemplar: cmd/exemplar/main.go pkg/exemplar/exemplar.go
	go build -o bin/exemplar cmd/exemplar/main.go

bin/step: cmd/step/main.go pkg/exemplar/exemplar.go
	go build -o bin/step cmd/step/main.go


slm-w2.sqlite: bin/prepare /tinystories/wordnetify-tinystories/w2.sqlite
	./bin/prepare --input-database /tinystories/wordnetify-tinystories/w2.sqlite --output-database slm-w2.sqlite

test:
	go test ./...

clean:
	rm -rf bin/prepare bin/exemplar

dbclean:
	rm -f slm-w2.sqlite

