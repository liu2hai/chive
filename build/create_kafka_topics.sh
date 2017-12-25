#!/bin/sh

cd /Users/slash/opensource/kafka_2.11-1.0.0

./bin/kafka-topics.sh  --create  --zookeeper  localhost:2181  --replication-factor 1  --partitions  1  --topic okex_quote_pub

./bin/kafka-topics.sh  --create  --zookeeper  localhost:2181  --replication-factor 1  --partitions  1  --topic okex_archer_req

./bin/kafka-topics.sh  --create  --zookeeper  localhost:2181  --replication-factor 1  --partitions  1  --topic okex_archer_rsp

./bin/kafka-topics.sh --list --zookeeper localhost:2181
