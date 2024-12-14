SENSE_ANNOTATED_TRAINING_DATA=/tinystories/wordnetify-tinystories/TinyStories.sqlite
SENSE_ANNOTATED_TEST_DATA=/tinystories/wordnetify-tinystories/w2.sqlite

#SENSE_ANNOTATED_TRAINING_DATA=tiny.sqlite
#SENSE_ANNOTATED_TEST_DATA=w2.sqlite

######################################################################

.PHONY: build run test clean dbclean training-docker-image prepdata

build: bin/prepare bin/train bin/report bin/showtree bin/validation bin/listnodes
	echo All built

bin/prepare: cmd/prepare/main.go
	go build -o bin/prepare cmd/prepare/main.go

bin/train: cmd/train/main.go pkg/exemplar/exemplar.go
	go build -o bin/train cmd/train/main.go

bin/report: cmd/report/main.go
	go build -o bin/report cmd/report/main.go

bin/showtree: cmd/showtree/main.go
	go build -o bin/showtree cmd/showtree/main.go

bin/validation: cmd/validation/main.go pkg/inference/inference.go
	go build -o bin/validation cmd/validation/main.go

bin/listnodes: cmd/listnodes/main.go
	go build -o bin/listnodes cmd/listnodes/main.go

######################################################################


# I copied this to /ultratree/language-model/tiny.sqlite -- not a great name
sense-annotated-training-dataframe.sqlite: bin/prepare $(SENSE_ANNOTATED_TRAINING_DATA)
	./bin/prepare --input-database $(SENSE_ANNOTATED_TRAINING_DATA) --output-database sense-annotated-training-dataframe.sqlite

unannotated-training-dataframe.sqlite: bin/prepare $(SENSE_ANNOTATED_TRAINING_DATA)
	./bin/prepare --input-database $(SENSE_ANNOTATED_TRAINING_DATA) --output-database unannotated-training-dataframe.sqlite --output-choice=hash


# I copied this to /ultratree/language-model/validation.sqlite -- a really terrible name
sense-annotated-test-dataframe.sqlite: bin/prepare $(SENSE_ANNOTATED_TEST_DATA)
	./bin/prepare --input-database $(SENSE_ANNOTATED_TEST_DATA) --output-database sense-annotated-test-dataframe.sqlite


unannotated-test-dataframe.sqlite: bin/prepare $(SENSE_ANNOTATED_TEST_DATA)
	./bin/prepare --input-database $(SENSE_ANNOTATED_TEST_DATA) --output-database unannotated-test-dataframe.sqlite --output-choice=hash

prepdata: sense-annotated-training-dataframe.sqlite sense-annotated-test-dataframe.sqlite unannotated-test-dataframe.sqlite unannotated-training-dataframe.sqlite


training-docker-image: bin/train Dockerfile.train
	docker build -t ultratree-train -f Dockerfile.train .

test:
	go test ./...

clean:
	rm -rf bin/prepare bin/exemplar bin/train

dbclean:
	rm -f sense-annotated-test-data.sqlite

