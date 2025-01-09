#!/bin/bash

( cd ~/ultratree-results ; git pull -q )

TZ=UTC
cd ~/ultrametric-trees

ENSEMBLE_MODEL="@"
for i in 1 2 3 4 5
do
    ./bin/evaluatemodel \
	-model /ultratree/language-model/sense-annotated${i}.sqlite \
	-model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
	-test-data-database /ultratree/language-model/testdata.sqlite \
	-run-description "Default daily $(date +%Y-%m-%d)" \
	-output-database ~/ultratree-results/inferences.sqlite
    ENSEMBLE_MODEL="$ENSEMBLE_MODEL,/ultratree/language-model/sense-annotated${i}.sqlite"
    ./bin/contextreport -input /ultratree/language-model/sense-annotated${i}.sqlite \
			-output ~/ultratree-results/inferences.sqlite
done

ENSEMBLE_MODEL=$(echo $ENSEMBLE_MODEL | sed 's/^@,//')

./bin/evaluatemodel \
    -model $ENSEMBLE_MODEL \
    -model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
    -test-data-database /ultratree/language-model/testdata.sqlite \
    -run-description "Default daily for ensemble model $(date +%Y-%m-%d)" \
    -output-database ~/ultratree-results/inferences.sqlite

./bin/evaluatemodel \
    -model /ultratree/language-model/unannotated-model1.sqlite \
    -model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
    -test-data-database /ultratree/language-model/testdata.sqlite \
    -run-description "Default daily for unnotated data model #1 $(date +%Y-%m-%d)" \
    -output-database ~/ultratree-results/inferences.sqlite
./bin/contextreport -input /ultratree/language-model/unannotated-model1.sqlite \
		    -output ~/ultratree-results/inferences.sqlite


for CARE in 10 100 10000
do
  ./bin/evaluatemodel \
    -model /ultratree/language-model/careful${CARE}.sqlite \
    -model-cutoff-time "$(date +'%Y-%m-%d %H:%M:%S')" \
    -test-data-database /ultratree/language-model/testdata.sqlite \
    -run-description "Default daily for care ${CARE} data model $(date +%Y-%m-%d)" \
    -output-database ~/ultratree-results/inferences.sqlite
done 

sqlite3 ~/ultratree-results/inferences.sqlite ".dump evaluation_runs" > ~/ultratree-results/evaluation_runs.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump inferences" > ~/ultratree-results/inferences.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump context_snapshots" > ~/ultratree-results/context_snapshots.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump context_usage" > ~/ultratree-results/context_usage.sql


./bin/report -db /ultratree/language-model/sense-annotated${i}.sqlite > ~/ultratree-results/training-results.csv
./bin/report -db /ultratree/language-model/unannotated-model1.sqlite > ~/ultratree-results/unannotated-model1-training-results.csv

cd ~/ultratree-results
git add evaluation_runs.sql
git add inferences.sql
git add context_snapshots.sql
git add context_usage.sql
git add training-results.csv
git add unannotated-model1-training-results.csv
git commit -q -m"Automatic updates $(date +%Y-%m-%d)"
git pull -q
git push -q
