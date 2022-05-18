#! /bin/bash

MANAGED_NAMESPACE=$1
MANAGEMENT_NAMESPACE=$2
MANAGED_KUBECONFIG=/home/go/src/github.com/stolostron/hypershift-deployment-controller/hosted.kubeconfig
MANAGEMENT_KUBECONFIG=/root/.kube/config

oc --kubeconfig=$MANAGEMENT_KUBECONFIG annotate mce multiclusterengine pause=false --overwrite

oc --kubeconfig=$MANAGEMENT_KUBECONFIG set image -n multicluster-engine deployment/ocm-controller ocm-controller=quay.io/zhujian/multicloud-manager:hosted-addon