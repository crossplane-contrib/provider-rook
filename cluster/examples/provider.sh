#!/usr/bin/env bash

export BASE64ENCODED_K8S_TOKEN=$(base64 ./secrets/token.txt | tr -d "\n")
export BASE64ENCODED_K8S_ENDPOINT=$(base64 ./secrets/endpoint.txt | tr -d "\n")
export BASE64ENCODED_K8S_CLUSTER_CA=$(base64 ./secrets/ca.txt | tr -d "\n")
sed "s/BASE64ENCODED_K8S_TOKEN/$BASE64ENCODED_K8S_TOKEN/g;s/BASE64ENCODED_K8S_ENDPOINT/$BASE64ENCODED_K8S_ENDPOINT/g;s/BASE64ENCODED_K8S_CLUSTER_CA/$BASE64ENCODED_K8S_CLUSTER_CA/g" provider.yaml | kubectl create -f -