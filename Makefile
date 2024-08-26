.PHONY: build run test clean

build: bin/prepare
	echo All built

bin/prepare: cmd/prepare/main.go
	go build -o bin/prepare cmd/prepare/main.go

slm-w2.sqlite: bin/prepare /tinystories/wordnetify-tinystories/w2.sqlite
	./bin/prepare --input-database /tinystories/wordnetify-tinystories/w2.sqlite --output-database slm-w2.sqlite

clean:
	rm -rf bin/prepare

dbclean:
	rm -f slm-w2.sqlite

