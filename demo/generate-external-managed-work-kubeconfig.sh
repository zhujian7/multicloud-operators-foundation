#! /bin/bash

MANAGED_NAMESPACE=$1
MANAGEMENT_NAMESPACE=$2
MANAGED_KUBECONFIG=/home/go/src/github.com/stolostron/hypershift-deployment-controller/hosted.kubeconfig
MANAGEMENT_KUBECONFIG=/root/.kube/config
MANAGED_BOOTSTRAPT_KUBECONFIG=external-managed-kubeconfig-registration

managed_addon_workmgr_kubeconfig_template_path=external-managed-work-kubeconfig.template
managed_addon_workmgr_kubeconfig_path=external-managed-work-kubeconfig

TEMP="/tmp/hosted-addon-kubeconfig-temp-test"
mkdir -p $TEMP

secret_name=""
for name in $(oc --kubeconfig=$MANAGED_KUBECONFIG get sa -n $MANAGED_NAMESPACE klusterlet-addon-workmgr -ojsonpath='{.secrets[*].name}');
do 
    echo $name;
    if [[ $name == *"token"* ]]; then
        echo "It's there! secret $name"
        secret_name=$name
        break
    fi
done

echo "secret_name: $secret_name"

export MANAGED_KUBE_API_SERVER=$(oc --kubeconfig=$MANAGEMENT_KUBECONFIG get secret -n $MANAGEMENT_NAMESPACE $MANAGED_BOOTSTRAPT_KUBECONFIG -ojsonpath={.data.kubeconfig} | base64 -d | grep "server: " | sed -e "s/server: //" | xargs)
export MANAGED_CA=$(oc --kubeconfig=$MANAGED_KUBECONFIG get secret -n $MANAGED_NAMESPACE $secret_name -ojsonpath={.data.ca\\.crt})
export MANAGED_TOKEN=$(oc --kubeconfig=$MANAGED_KUBECONFIG get secret -n $MANAGED_NAMESPACE $secret_name -ojsonpath={.data.token} | base64 -d)


echo "MANAGED_KUBE_API_SERVER: $MANAGED_KUBE_API_SERVER"
(envsubst < $managed_addon_workmgr_kubeconfig_template_path) > $managed_addon_workmgr_kubeconfig_path

rm -rf $TEMP



oc --kubeconfig=$MANAGEMENT_KUBECONFIG delete secret external-work-manager-addon-kubeconfig -n $MANAGEMENT_NAMESPACE
oc --kubeconfig=$MANAGEMENT_KUBECONFIG create secret generic external-work-manager-addon-kubeconfig -n $MANAGEMENT_NAMESPACE \
  --from-file=kubeconfig=$managed_addon_workmgr_kubeconfig_path

echo "MANAGED_TOKEN: $MANAGED_TOKEN"