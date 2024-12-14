#!/bin/bash

TZ=UTC
cd ~/ultrametric-trees
./bin/validation \
    -model /ultratree/language-model/tiny.sqlite \
    -model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
    -validation-database /ultratree/language-model/validation.sqlite \
    -run-description "Default daily $(date +%Y-%m-%d)" \
    -output-database ~/ultratree-results/inferences.sqlite

./bin/validation \
    -model /ultratree/language-model/unannotated-model1.sqlite \
    -model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
    -validation-database /ultratree/language-model/validation.sqlite \
    -run-description "Default daily for unnotated data model #1 $(date +%Y-%m-%d)" \
    -output-database ~/ultratree-results/unannotated-model1.sqlite

sqlite3 ~/ultratree-results/inferences.sqlite ".dump validation_runs" > ~/ultratree-results/validation_runs.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump inferences" > ~/ultratree-results/inferences.sql

sqlite3 ~/ultratree-results/unannotated-model1.sqlite ".dump validation_runs" > ~/ultratree-results/unannotated-model1-validation_runs.sql
sqlite3 ~/ultratree-results/unannotated-model1.sqlite ".dump inferences" > ~/ultratree-results/unannotated-model1-inferences.sql


./bin/report -db /ultratree/language-model/tiny.sqlite > ~/ultratree-results/training-results.csv
./bin/report -db /ultratree/language-model/unannotated-model1.sqlite > ~/ultratree-results/unannotated-model1-training-results.csv

cd ~/ultratree-results
git pull -q
git add validation_runs.sql
git add inferences.sql
git add training-results.csv
git add unannotated-model1-training-results.csv
git commit -q -m"Automatic updates $(date +%Y-%m-%d)"
git push -q
