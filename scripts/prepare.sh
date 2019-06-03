#!/bin/bash

# Make sure drand works!

# Copy over volumes
mkdir -p /root/.drand/key
chmod 740 /root/.drand/key
cat /root/.drand/key-volume/drand_id.private > /root/.drand/key/drand_id.private
cat /root/.drand/key-volume/drand_id.public > /root/.drand/key/drand_id.public
cat /root/.drand/group-volume/group.toml > /root/.drand/group.toml
cat /root/.drand/preexisting-volume/already_running_nodes.toml > /root/.drand/preexisting-group.toml

# Pull cached data from s3
s3cmd sync s3://drand/$1/db/ /root/.drand/db/ --recursive --delete-removed
s3cmd sync s3://drand/$1/groups/ /root/.drand/groups/ --recursive --delete-removed

# Start drand daemon.
if [ -e /root/.drand/db ]; then
    echo "n" | /drand -V 2 start --tls-disable --listen 0.0.0.0:$2 --control $3 &
else
    /drand -V 2 start --tls-disable --listen 0.0.0.0:$2 --control $3 &
fi


# If drand already has cached directory, do not re-run DKG. Else re-run DKG.
if [ -e /root/.drand/groups/dist_key.public ]
then 
    echo "drand cluster configuration already exists; resuming operation with preexisting drand info"
else
    echo "will have to run DKG to generate new drand cluster configuration"
    # Wait for drand daemon to get set up (and if leader, sleep long enough so other 
    # drand nodes are up before starting distrubted key generation).
    if [ $4 -eq 1 ]; then # Node is not running as leader and needs to start DKG from scratch
        sleep 3
        /drand -V 2 share /root/.drand/group.toml --control $3
    elif [ $4 -eq 2 ]; then # Node is joining preexisting drand cluster.
        sleep 3
        /drand -V 2 share --from /root/.drand/preexisting-group.toml /root/.drand/group.toml --control $3
    else # Node is running as leader
        sleep 15
        /drand -V 2 share --leader /root/.drand/group.toml --control $3
    fi
fi

# Start node prometheus exporter
prometheus-node-exporter --web.listen-address=0.0.0.0:$5 > /var/log/prometheus-node-exporter.log
