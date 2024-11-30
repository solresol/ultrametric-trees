#!/bin/bash

docker run --detach=true -v /ultratree/language-model:/ultratree/language-model ultratree-train \
       --database /ultratree/language-model/tiny.sqlite \
       --solar-monitor envoy.cassia.ifost.org.au

# one day I might add --restart=always
