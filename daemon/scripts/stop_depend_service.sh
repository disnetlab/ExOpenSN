#!/bin/bash
set +e
sudo docker rm -f opensn_etcd
sudo docker rm -f opensn_influxdb
sudo docker rm -f opensn_codeserver
