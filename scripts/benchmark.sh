#!/bin/bash

BPLOG_ROOT="$GOPATH/src/github.com/bluemedora/bplogagent"

run_time=$(date +%Y-%m-%d-%H-%M-%S)

PROJECT="bindplane-agent-dev-0"
ZONE="us-central1-a"
INSTANCE="rhel6-$run_time"

if ! [ -x "$(command -v gcloud)" ]; then
  echo 'Error: gcloud is not installed.' >&2
  exit 1
fi

if ! [ -x "$(command -v dot)" ]; then
  echo 'Error: dot is not installed.' >&2
  exit 1
fi

echo "Creating Instance: $INSTANCE [$PROJECT] [$ZONE]"
gcloud beta compute instances create --verbosity=error \
  --project=$PROJECT --zone=$ZONE $INSTANCE --preemptible \
  --image=rhel-6-v20200402 --image-project=rhel-cloud \
  --machine-type=n1-standard-1 --boot-disk-size=20GB > /dev/null

echo "Waiting for instance to be ready"
until gcloud beta compute ssh --project $PROJECT --zone $ZONE $INSTANCE --ssh-flag="-o LogLevel=QUIET" -- 'echo "Ready"  2>&1 > /dev/null'; do
  echo "VM not ready. Waiting..."  
done

echo "Building log agent"
GOOS=linux go build -o /tmp/bplogagent $BPLOG_ROOT/cmd

echo "Building benchmark tool"
GOOS=linux go build -o /tmp/logbench github.com/awslabs/amazon-log-agent-benchmark-tool/cmd/logbench/

echo "Setting up benchmark test"
gcloud beta compute ssh --project $PROJECT --zone $ZONE $INSTANCE --ssh-flag="-o LogLevel=QUIET" -- 'rm -rf ~/benchmark && mkdir ~/benchmark' > /dev/null
gcloud beta compute scp --project $PROJECT --zone $ZONE /tmp/bplogagent $INSTANCE:~/benchmark > /dev/null
gcloud beta compute scp --project $PROJECT --zone $ZONE /tmp/logbench $INSTANCE:~/benchmark/LogBench > /dev/null
gcloud beta compute scp --project $PROJECT --zone $ZONE $BPLOG_ROOT/scripts/benchmark/config.yaml $INSTANCE:~/benchmark/config.yaml > /dev/null
gcloud beta compute ssh --project $PROJECT --zone $ZONE $INSTANCE --ssh-flag="-o LogLevel=QUIET" -- 'chmod -R 777 ~/benchmark' > /dev/null

echo "Running single-file benchmark (60 seconds)"
gcloud beta compute ssh --project $PROJECT --zone $ZONE $INSTANCE --ssh-flag="-o LogLevel=QUIET" -- \
  'set -m
  ~/benchmark/LogBench -log stream.log -rate 100 -t 60s -r 30s -f 2s ~/benchmark/bplogagent --config ~/benchmark/config.yaml > ~/benchmark/output1 2>&1 &
  sleep 10;
  curl http://localhost:6060/debug/pprof/profile?seconds=30 > ~/benchmark/profile1 ; 
  fg ; ' > /dev/null

echo "Running 20-file benchmark (60 seconds)"
gcloud beta compute ssh --project $PROJECT --zone $ZONE $INSTANCE --ssh-flag="-o LogLevel=QUIET" -- \
  'set -m
  ~/benchmark/LogBench -log $(echo stream{1..20}.log | tr " " ,) -rate 100 -t 60s -r 30s -f 2s ~/benchmark/bplogagent --config ~/benchmark/config.yaml > ~/benchmark/output20 2>&1 &
  sleep 10;
  curl http://localhost:6060/debug/pprof/profile?seconds=30 > ~/benchmark/profile20 ; 
  fg ; ' > /dev/null

output_dir="$BPLOG_ROOT/tmp/$run_time"
mkdir -p $output_dir

echo "Retrieving results"
gcloud beta compute scp --project $PROJECT --zone $ZONE $INSTANCE:~/benchmark/output* $output_dir > /dev/null
gcloud beta compute scp --project $PROJECT --zone $ZONE $INSTANCE:~/benchmark/profile* $output_dir > /dev/null

echo "Cleaning up instance"
gcloud beta compute instances delete --quiet --project $PROJECT --zone=$ZONE $INSTANCE

echo
echo "Output files"
echo "  $output_dir/output1"
echo "  $output_dir/output20"

echo
echo "Profiles can be accessed with the following commands"
echo "  go tool pprof -http localhost:6001 $output_dir/profile1"
echo "  go tool pprof -http localhost:6020 $output_dir/profile20"