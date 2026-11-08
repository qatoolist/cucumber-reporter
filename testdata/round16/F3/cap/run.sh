#!/bin/zsh
# usage: run.sh name "ENV=.. ENV2=.." cucumber-args...
W=${0:A:h}; name=$1; envs=$2; shift 2
d=$W/$name; rm -rf $d; mkdir -p $d; cp -R $W/proj/features $d/
cd $d
env CUCUMBER_PUBLISH_QUIET=true ${=envs} cucumber --no-color "$@" --format progress --out summary.txt --format json --out run.json --format message --out run.ndjson > stdout.txt 2> stderr.txt
code=$?
echo "\$ $envs cucumber $*" >> summary.txt; echo "exit=$code" >> summary.txt
echo "$name exit=$code $(tail -n 4 summary.txt | head -n 2 | tr '\n' ' ')"
