#!/bin/bash

( cd ~/ultratree-results ; git pull -q )

TZ=UTC

export ULTRATREE_EVAL_RUN_DESCRIPTION="Default daily $(date +%Y-%m-%d)"
export ULTRATREE_EVAL_TEST_DATA_DB_PATH="/ultratree/language-model/testdata.sqlite"
export ULTRATREE_EVAL_MODEL_CUTOFF_TIME="$(date +'%Y-%m-%d %H:%M:%S')"
export ULTRATREE_EVAL_OUTPUT_DB_PATH=~/ultratree-results/inferences.sqlite
#export ULTRATREE_EVAL_OUTPUT_DB_PATH=/tmp/inferences.sqlite

# There is another environment variable I could use:
#  export ULTRATREE_EVAL_MODEL_PATHS=...
# But since that is different for each evaluation run here, I've chosen
# not to set it.

cd ~/ultrametric-trees


ENSEMBLE_MODEL="@"
for i in 1 2 3 4 5
do
    ./bin/evaluatemodel -model /ultratree/language-model/sense-annotated${i}.sqlite
    ENSEMBLE_MODEL="$ENSEMBLE_MODEL,/ultratree/language-model/sense-annotated${i}.sqlite"
done

ENSEMBLE_MODEL=$(echo $ENSEMBLE_MODEL | sed 's/^@,//')

./bin/evaluatemodel -model $ENSEMBLE_MODEL
./bin/evaluatemodel -model /ultratree/language-model/unannotated-model1.sqlite

for CARE in 10 100 10000
do
  ./bin/evaluatemodel -model /ultratree/language-model/careful${CARE}.sqlite
done


for i in 1 2 3 4 5
do
    ./bin/contextreport -input /ultratree/language-model/sense-annotated${i}.sqlite \
			-output $ULTRATREE_EVAL_OUTPUT_DB_PATH
done

./bin/contextreport -input /ultratree/language-model/unannotated-model1.sqlite \
		    -output $ULTRATREE_EVAL_OUTPUT_DB_PATH


for CARE in 10 100 10000
do
    ./bin/contextreport -input /ultratree/language-model/careful${CARE}.sqlite \
			-output ~/ultratree-results/inferences.sqlite    
done 


sqlite3 ~/ultratree-results/inferences.sqlite ".dump evaluation_runs" > ~/ultratree-results/evaluation_runs.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump inferences" > ~/ultratree-results/inferences.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump context_snapshots" > ~/ultratree-results/context_snapshots.sql
sqlite3 ~/ultratree-results/inferences.sqlite ".dump context_usage" > ~/ultratree-results/context_usage.sql
rm -f ~/ultratree-results/inferences.sql.gz
gzip -9 ~/ultratree-results/inferences.sql

./bin/report -db /ultratree/language-model/sense-annotated${i}.sqlite > ~/ultratree-results/training-results.csv
./bin/report -db /ultratree/language-model/unannotated-model1.sqlite > ~/ultratree-results/unannotated-model1-training-results.csv

cd ~/ultratree-results
git add evaluation_runs.sql
git add inferences.sql.gz
git add context_snapshots.sql
git add context_usage.sql
git add training-results.csv
git add unannotated-model1-training-results.csv
git commit -q -m"Automatic updates $(date +%Y-%m-%d)"
git pull -q
git push -q
