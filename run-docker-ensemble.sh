#!/bin/bash

docker rm sense-annotated-train2
cp sense-annotated-training-dataframe.sqlite /ultratree/language-model/sense-annotated2.sqlite
docker run --name=sense-annotated-train2 \
       --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/sense-annotated2.sqlite \
       --solar-monitor envoy.cassia.ifost.org.au --seed 2

docker rm sense-annotated-train3
cp sense-annotated-training-dataframe.sqlite /ultratree/language-model/sense-annotated3.sqlite
docker run --name=sense-annotated-train3 \
       --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/sense-annotated3.sqlite \
       --solar-monitor envoy.cassia.ifost.org.au --seed 3

docker rm sense-annotated-train4
cp sense-annotated-training-dataframe.sqlite /ultratree/language-model/sense-annotated4.sqlite
docker run --name=sense-annotated-train4 \
       --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/sense-annotated4.sqlite \
       --solar-monitor envoy.cassia.ifost.org.au --seed 4

docker rm sense-annotated-train5
cp sense-annotated-training-dataframe.sqlite /ultratree/language-model/sense-annotated5.sqlite
docker run --name=sense-annotated-train5 \
       --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/sense-annotated5.sqlite \
       --solar-monitor envoy.cassia.ifost.org.au --seed 5

