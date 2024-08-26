.PHONY: build run test clean

build: bin/traverse bin/generate
	echo All built

bin/traverse: cmd/traverse/main.go internal/traverse/traverse.go
	go build -o bin/traverse cmd/traverse/main.go

bin/generate:
	go build -o bin/generate cmd/generate/main.go


#.wordnet.sqlite: bin/traverse dict/index.noun dict/index.verb
#	./bin/traverse -wordnet dict -sqlite .wordnet.sqlite

.wordnet.sqlite: make_wordnet_database.py
	python3 make_wordnet_database.py --sqlite .wordnet.sqlite

test:
	go test ./...

clean:
	rm -rf bin/traverse bin/generate

run: .wordnet.sqlite
	./bin/generate -input input.db -traverse traverse.db -output output.db

wn3.1.dict.tar.gz:
	wget https://wordnetcode.princeton.edu/wn3.1.dict.tar.gz

dict/index.noun dict/index.verb: wn3.1.dict.tar.gz
	tar xvfz wn3.1.dict.tar.gz
