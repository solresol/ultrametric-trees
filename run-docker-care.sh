#!/bin/bash

docker rm careful10000-train1
cp sense-annotated-training-dataframe.sqlite /ultratree/language-model/careful10000.sqlite
docker run --name=careful10000 \
       --restart unless-stopped \
       --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/careful10000.sqlite \
       --exemplar-guesses 10000 \
       --cost-guesses 10000 \
       --split-count-try 1000 \
       --solar-monitor envoy.cassia.ifost.org.au --seed 1

docker rm careful100-train1
cp sense-annotated-training-dataframe.sqlite /ultratree/language-model/careful100.sqlite
docker run --name=careful100 \
       --restart unless-stopped \
       --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/careful100.sqlite \
       --exemplar-guesses 100 \
       --cost-guesses 100 \
       --split-count-try 10 \
       --solar-monitor envoy.cassia.ifost.org.au --seed 1

docker rm careful10-train1
cp sense-annotated-training-dataframe.sqlite /ultratree/language-model/careful10.sqlite
docker run --name=careful10 \
       --restart unless-stopped \
       --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/careful10.sqlite \
       --exemplar-guesses 10 \
       --cost-guesses 10 \
       --split-count-try 10 \
       --solar-monitor envoy.cassia.ifost.org.au --seed 1
