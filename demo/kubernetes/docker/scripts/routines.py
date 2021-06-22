#!/usr/bin/python3 -u
import os, sys, subprocess
import time

__config_file        = os.environ.get('KIND_CLUSTER_MANIFEST', '')
__config_kubeconfig  = os.environ.get('KIND_KUBECONFIG', '/root/.kube/config')
__config_manifests   = os.environ.get('KIND_INITIAL_KUBERNETES_MANIFESTS', '/manifests/init.yml')
__config_dns_address = os.environ.get('KIND_DNS_SERVER_IP', '10.254.254.254')

def help(exitcode=0):
    print("routines.py - control cluster bootstraping and destroying")
    print("")
    print("Usage:")
    print("  create - bootstraps cluster")
    print("  delete - destroys cluster")
    print("  ENVS:")
    print("      KIND_CLUSTER_MANIFEST             - path to cluster manifest file")
    print("      KIND_KUBECONFIG                   - path to kubeconfig to OVERRIDE cluster access data")
    print("      KIND_INITIAL_KUBERNETES_MANIFESTS - path to kubernetes manifests for initial")
    print("                                          installation (kubernetes dashboard by default)")
    print("      KIND_DNS_SERVER_IP                - ip address of DNS server")
    sys.exit(exitcode)

def create():
    if __config_file == '':
        print("You must define KIND_CLUSTER_MANIFEST env to path of cluster manifest.")
        print("See details here: https://kind.sigs.k8s.io/docs/user/configuration/")
        print("")
        help(exitcode=4)

    # Wipe kubeconfig
    with open(__config_kubeconfig, 'w') as fd:
        fd.write('')

    # Create cluster
    proc = subprocess.Popen([
        "/usr/local/bin/kind",
        "create",
        "cluster",
        "--config",
        __config_file
    ], stdout=sys.stdout, stderr=sys.stderr, universal_newlines=True)
    proc.communicate()

    # Refactor kubeconfig
    for pattern in ['.clusters[0].cluster.server = "https://kubernetes:6443"']: # '.clusters[0].cluster."insecure-skip-tls-verify" = true'
        proc = subprocess.Popen([
            "/usr/bin/yq",
            "-i",
            "-y",
            pattern,
            __config_kubeconfig,
        ], stdout=sys.stdout, stderr=sys.stderr, universal_newlines=True)
        proc.communicate()

    time.sleep(5)

    # Owerride dns config for nodes
    proc = subprocess.Popen([
        "/usr/local/bin/docker",
        "exec",
        "-i",
        "kind-control-plane",
        "sh",
        "-c",
        "echo nameserver " + __config_dns_address + " > /etc/resolv.conf",
    ], stdout=sys.stdout, stderr=sys.stderr, universal_newlines=True)
    proc.communicate()

    # Apply few manifests
    proc = subprocess.Popen([
        "/usr/local/bin/kubectl",
        "create",
        "-f",
        __config_manifests,
    ], stdout=sys.stdout, stderr=sys.stderr, universal_newlines=True)
    proc.communicate()

def delete():
    # Delete cluster
    proc = subprocess.Popen([
        "/usr/local/bin/kind",
        "delete",
        "cluster",
    ], stdout=sys.stdout, stderr=sys.stderr, universal_newlines=True)
    proc.communicate()

def main():
    if len(sys.argv) != 2:
        help(exitcode=2)

    if sys.argv[1] == "create":
        create()
    elif sys.argv[1] == "delete":
        delete()
    elif sys.argv[1] == "help":
        help()
    else:
        print("bad command")
        help(exitcode=3)

if __name__ == "__main__":
    main()
