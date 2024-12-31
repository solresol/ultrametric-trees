#!/bin/bash

# To-do: all the other scripts shouldn't do a run, they should
# just do a create, and then run this script.

docker start unannotated-model1
docker start sense-annotated-train1 
docker start sense-annotated-train2
docker start sense-annotated-train3
docker start sense-annotated-train4
docker start sense-annotated-train5

docker start careful10000
docker start careful100 
docker start careful10
