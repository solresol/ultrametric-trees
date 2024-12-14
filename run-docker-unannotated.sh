#!/bin/bash

docker run --name unannotated-model1 --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/unannotated-model1.sqlite \
       --solar-monitor envoy.cassia.ifost.org.au

# one day I might add --restart=always
