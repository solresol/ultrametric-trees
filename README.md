# ultrametric-trees
Training decision trees that minimise an ultrametric


# WordNet Traversal

This project traverses the WordNet database and stores synset paths in a SQLite database.

## Prerequisites

- Go 1.16 or later
- WordNet database files

## Building

To build the project, run:

```
make build
```

This will create the `traverse` binary in the `bin/` directory.

## Running

To run the program:

```
make run
```

Or manually:

```
./bin/traverse -wordnet /path/to/wordnet/dict -sqlite /path/to/output/wordnet.db
```

## Testing

To run tests:

```
make test
```

## Cleaning

To clean build artifacts:

```
make clean
```
