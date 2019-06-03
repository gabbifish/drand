#!/bin/bash

cp /dist_key.private ~/.drand/groups/ 
cp /dist_key.public ~/.drand/groups/ 
cp /drand_group.toml ~/.drand/groups/ 
s3cmd sync /root/.drand/groups/ s3://drand/$1/groups/ --recursive --delete-removed
s3cmd sync /root/.drand/db/ s3://drand/$1/db/ --recursive --delete-removed