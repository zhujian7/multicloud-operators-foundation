#! /bin/bash

MANAGED_NAMESPACE=$1
MANAGEMENT_NAMESPACE=$2
MANAGED_KUBECONFIG=/home/go/src/github.com/stolostron/hypershift-deployment-controller/hosted.kubeconfig
MANAGEMENT_KUBECONFIG=/root/.kube/config


TEMP="/tmp/hosted-addon-temp-test"
mkdir -p $TEMP


oc --kubeconfig=$MANAGED_KUBECONFIG get secret -n $MANAGED_NAMESPACE work-manager-hub-kubeconfig -ojsonpath="{.data.kubeconfig}" | base64 -d > $TEMP/kubeconfig
oc --kubeconfig=$MANAGED_KUBECONFIG get secret -n $MANAGED_NAMESPACE work-manager-hub-kubeconfig -ojsonpath="{.data.tls\\.crt}" | base64 -d > $TEMP/tls.crt
oc --kubeconfig=$MANAGED_KUBECONFIG get secret -n $MANAGED_NAMESPACE work-manager-hub-kubeconfig -ojsonpath="{.data.tls\\.key}" | base64 -d > $TEMP/tls.key

oc --kubeconfig=$MANAGEMENT_KUBECONFIG delete secret work-manager-hub-kubeconfig -n $MANAGEMENT_NAMESPACE
# oc --kubeconfig=$MANAGEMENT_KUBECONFIG create secret generic work-manager-hub-kubeconfig -n $MANAGEMENT_NAMESPACE \
#   --from-file=kubeconfig=$TEMP/kubeconfig --from-file=tls.crt=$TEMP/tls.crt --from-file=tls.key=$TEMP/tls.key 

oc --kubeconfig=$MANAGEMENT_KUBECONFIG create secret generic work-manager-hub-kubeconfig -n $MANAGEMENT_NAMESPACE \
  --from-file=kubeconfig=$MANAGEMENT_KUBECONFIG

#oc --kubeconfig=$MANAGEMENT_KUBECONFIG get pod -n $MANAGEMENT_NAMESPACE| grep klusterlet-addon-workmgr | awk '{print $1}' | xargs oc --kubeconfig=$MANAGEMENT_KUBECONFIG delete pod -n $MANAGEMENT_NAMESPACE

rm -rf $TEMP
