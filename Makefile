.PHONY: build run test clean dbclean training-docker-image

build: bin/prepare bin/train bin/report bin/showtree bin/validation bin/listnodes
	echo All built

bin/prepare: cmd/prepare/main.go
	go build -o bin/prepare cmd/prepare/main.go

bin/train: cmd/train/main.go pkg/exemplar/exemplar.go pkg/node/node.go
	go build -o bin/train cmd/train/main.go

bin/report: cmd/report/main.go
	go build -o bin/report cmd/report/main.go

bin/showtree: cmd/showtree/main.go
	go build -o bin/showtree cmd/showtree/main.go

bin/validation: cmd/validation/main.go pkg/inference/inference.go
	go build -o bin/validation cmd/validation/main.go

bin/listnodes: cmd/listnodes/main.go
	go build -o bin/listnodes cmd/listnodes/main.go

slm-w2.sqlite: bin/prepare /tinystories/wordnetify-tinystories/w2.sqlite
	./bin/prepare --input-database /tinystories/wordnetify-tinystories/w2.sqlite --output-database slm-w2.sqlite

tiny.sqlite: bin/prepare /tinystories/wordnetify-tinystories/TinyStories.sqlite
	./bin/prepare --input-database /tinystories/wordnetify-tinystories/TinyStories.sqlite --output-database /ultratree/language-model/tiny.sqlite

training-docker-image: bin/train Dockerfile.train
	docker build -t ultratree-train -f Dockerfile.train .

test:
	go test ./...

clean:
	rm -rf bin/prepare bin/exemplar

dbclean:
	rm -f slm-w2.sqlite

