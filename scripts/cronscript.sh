#!/bin/bash

TZ=UTC
cd ~/ultrametric-trees

#./bin/evaluatemodel \
#    -model /ultratree/language-model/tiny.sqlite \
#    -model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
#    -test-data-database /ultratree/language-model/testdata.sqlite \
#    -run-description "Default daily $(date +%Y-%m-%d)" \
#    -output-database ~/ultratree-results/inferences.sqlite

for i in 1 2 3 4 5
do
    ./bin/evaluatemodel \
	-model /ultratree/language-model/sense-annotated${i}.sqlite \
	-model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
	-test-data-database /ultratree/language-model/testdata.sqlite \
	-run-description "Default daily $(date +%Y-%m-%d)" \
	-output-database ~/ultratree-results/inferences.sqlite    
done

./bin/evaluatemodel \
    -model /ultratree/language-model/unannotated-model1.sqlite \
    -model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
    -test-data-database /ultratree/language-model/testdata.sqlite \
    -run-description "Default daily for unnotated data model #1 $(date +%Y-%m-%d)" \
    -output-database ~/ultratree-results/inferences.sqlite

sqlite3 ~/ultratree-results/inferences.sqlite ".dump evaluation_runs" > ~/ultratree-results/evaluation_runs.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump inferences" > ~/ultratree-results/inferences.sql


./bin/report -db /ultratree/language-model/sense-annotated${i}.sqlite > ~/ultratree-results/training-results.csv
./bin/report -db /ultratree/language-model/unannotated-model1.sqlite > ~/ultratree-results/unannotated-model1-training-results.csv

cd ~/ultratree-results
git pull -q
git add evaluation_runs.sql
git add inferences.sql
git add training-results.csv
git add unannotated-model1-training-results.csv
git commit -q -m"Automatic updates $(date +%Y-%m-%d)"
git push -q
