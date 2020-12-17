# dummy-cni
This is a simple plugin used for vpp applications. We needed a way to give ip address info to pod using multus/sriov however since VPP attaches directly to the network interface we needed a method for the application to get the ip info without having to tear down an interface.

# prerequisite
for this example we will need 3 other CNI's
  * [Multus](https://github.com/intel/multus-cni/)
  * [SRIOV](https://github.com/k8snetworkplumbingwg/sriov-cni)
  * [Whereabouts](https://github.com/dougbtv/whereabouts)

you can install all 3 of these with helm3 (helm 2 is not supported)

this also assumes you are using the following sriov config

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: sriov
    meta.helm.sh/release-namespace: kube-system
  labels:
    app.kubernetes.io/managed-by: Helm
  name: sriov-sriov-0.1.0-config
  namespace: kube-system
data:
  dp-conf.json: |-
    {
    "resourceList": [
        {
        "resourceName": "mlnx_sriov_PF_1",
        "resourcePrefix": "mellanox.com",
        "selectors": {
            "pfnames": [
            "ens1f0"
            ]
        }
        },
        {
        "resourceName": "mlnx_sriov_PF_2",
        "resourcePrefix": "mellanox.com",
        "selectors": {
            "pfnames": [
            "ens1f1"
            ]
        }
        }
    ]
    }
```

```
git clone https://github.com/k8snetworkplumbingwg/helm-charts.git
cd helm-charts
helm upgrade --install multus ./multus  --namespace kube-system
helm upgrade --install sriov ./sriov  --namespace kube-system
helm upgrade --install whereabouts ./whereabouts --namespace kube-system
```
time to build the docker image
```
export IMAGE_REPO=<name_of_your_docker_repo>
make docker-build
make docker-push
```
now we can install the dummy-cni
```
cat <<EOF | kubectl appy -f -
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: dummy-cni
  namespace: kube-system
  labels:
    app: dummy-cni
spec:
  selector:
    matchLabels:
      name: dummy-cni
  template:
    metadata:
      labels:
        name: dummy-cni
    spec:
      tolerations:
        - operator: Exists
          effect: NoSchedule
      containers:
      - name: dummy-cni
        imagePullPolicy: Always
        image: ${IMAGE_REPO)/dummy-cni:v0.1
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 200Mi
        volumeMounts:
        - mountPath: /host/opt/cni/bin
          name: cnibin
      volumes:
      - hostPath:
          path: /opt/cni/bin
        name: cnibin
EOF
```

now we can create our `NetworkAttachmentDefinition`s

```
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: mellanox.com/mlnx_sriov_PF_1
  name: sriov-vlan592-1
  namespace: lightning
spec:
  config: '{
        "type": "sriov",
        "cniVersion": "0.3.1",
        "vlan": 592,
        "name": "sriov-network",
        "spoofchk":"off",
}'
EOF
```

```
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: mellanox.com/mlnx_sriov_PF_1
  name: sriov-vlan592-2
  namespace: lightning
spec:
  config: '{
      "type": "sriov",
      "cniVersion": "0.3.1",
      "vlan": 592,
      "name": "sriov-network",
      "spoofchk":"off"
}'
EOF
```

```
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: netskope.io/dummy
  name: dummy
  namespace: lightning
spec:
  config: '{
      "type": "dummy-cni",
      "cniVersion": "0.3.1",
      "name": "dummy-network",
      "ifname": "dummy0",
      "ipam": {
        "gateway": "10.115.251.1",
        "type": "whereabouts",
        "datastore": "kubernetes",
        "kubernetes": {
            "kubeconfig": "/etc/cni/net.d/whereabouts.d/whereabouts.kubeconfig"
        },
        "range": "10.115.251.2-10.115.251.254/24",
        "log_file" : "/tmp/whereabouts.log",
        "log_level" : "debug"
        }
    }'
EOF
```
now lets test this all out and create a pod that requests all these interfaces

```
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  annotations:
    k8s.v1.cni.cncf.io/networks: '[
{"name": "sriov-vlan592-1",
"interface": "net1"
},
{"name": "sriov-vlan592-2",
"interface": "net2"
},
{"name": "dummy",
"interface": "dummy0"
}]'
  name: dummypod
  namespace: lightning
spec:
  containers:
  - args:
    - while true; do sleep 300000; done;
    command:
    - /bin/bash
    - -c
    - --
    image: nginx:latest
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        mellanox.com/mlnx_sriov_PF_1: '1'
        mellanox.com/mlnx_sriov_PF_2: '1'
      limits:
        mellanox.com/mlnx_sriov_PF_1: '1'
        mellanox.com/mlnx_sriov_PF_2: '1'
    name: nginx
    securityContext:
      privileged: true
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
  dnsPolicy: ClusterFirst
  enableServiceLinks: true
  priority: 0
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: 300
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
    tolerationSeconds: 300
EOF
```