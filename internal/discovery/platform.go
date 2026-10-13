package discovery

import (
	"context"
	"os"
	"strings"
	"sync"
)

// What this gateway runs ON, for the procedures the console prints.
//
// Moving the database ends with a change to the deployment - the gateway picks
// its database at start, from MEERKAT_DATABASE_URL - and the deployment files
// stay the one truth about it. So the console does not keep a pointer of its
// own: it prints the change for THIS platform, with this installation's names,
// and the change is the operator's to make. A Helm release wants a Secret and
// a helm upgrade, a Swarm service a service update and the line that keeps the
// stack file in step, a compose project two lines of environment. Written for
// the wrong one, a procedure is a puzzle.

// Platform is what the runtime says about this gateway's own deployment. Name
// is one of kubernetes, openshift, swarm, compose, docker (a container whose
// runtime could not be asked further) and binary (no container at all).
type Platform struct {
	Name string `json:"name"`
	// Helm says a Helm release manages this gateway. HelmAssumed says it is
	// assumed rather than seen: the pod could not be read, and the chart is
	// how a gateway is installed on a cluster.
	Helm        bool   `json:"helm,omitempty"`
	HelmAssumed bool   `json:"helmAssumed,omitempty"`
	Release     string `json:"release,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	// Workload is the thing to change: the Deployment, the Swarm service, the
	// compose service, the container.
	Workload string `json:"workload,omitempty"`
	// Stack is the Swarm stack, or the compose project.
	Stack string `json:"stack,omitempty"`
	// Files and Dir are a compose project's files and the directory it is run
	// from, as compose labelled the container; Container is the container
	// itself, which docker cp reaches even stopped.
	Files     []string `json:"files,omitempty"`
	Dir       string   `json:"dir,omitempty"`
	Container string   `json:"container,omitempty"`
	// Why says what could not be read, when something could not.
	Why string `json:"why,omitempty"`
}

var (
	platformMu   sync.Mutex
	platformSeen *Platform
)

// Runtime asks the runtime what this gateway runs on. A complete answer is
// kept for the life of the process - a gateway does not change platform while
// it runs - and an incomplete one is asked again next time.
func Runtime(ctx context.Context) Platform {
	platformMu.Lock()
	defer platformMu.Unlock()
	if platformSeen != nil {
		return *platformSeen
	}
	p := readPlatform(ctx)
	if p.Why == "" {
		platformSeen = &p
	}
	return p
}

func readPlatform(ctx context.Context) Platform {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		openshift := false
		if _, err := os.Stat(saDir + "/service-ca.crt"); err == nil {
			openshift = true
		}
		self, err := readK8sSelf(ctx)
		if err != nil {
			p := k8sPlatform(nil, "", "", openshift)
			p.Why = "could not read this gateway's pod: " + err.Error()
			return p
		}
		host, _ := os.Hostname()
		return k8sPlatform(self.labels, host, self.namespace, openshift)
	}
	client, base, err := dockerClient()
	if err != nil {
		if inContainer() {
			return Platform{Name: "docker", Why: "a container, with no Docker socket to ask which service it belongs to"}
		}
		return Platform{Name: "binary"}
	}
	host, _ := os.Hostname()
	var c struct {
		Name   string `json:"Name"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := getJSON(ctx, client, base+"/containers/"+host+"/json", &c); err != nil {
		// A Docker socket on a host that is not a container - a developer's
		// laptop, a server running the binary beside its containers - does
		// not know this process, and should not.
		if !inContainer() {
			return Platform{Name: "binary"}
		}
		return Platform{Name: "docker", Why: "the Docker socket does not know this container: " + err.Error()}
	}
	return dockerPlatform(c.Config.Labels, c.Name)
}

// k8sPlatform reads a pod's labels. Nil labels are a pod that could not be
// read: Helm is then assumed, the chart being how a gateway gets there.
func k8sPlatform(labels map[string]string, podName, namespace string, openshift bool) Platform {
	p := Platform{Name: "kubernetes", Namespace: namespace}
	if openshift {
		p.Name = "openshift"
	}
	if labels == nil {
		p.Helm, p.HelmAssumed = true, true
		return p
	}
	instance := labels["app.kubernetes.io/instance"]
	switch {
	case labels["app.kubernetes.io/managed-by"] == "Helm", labels["helm.sh/chart"] != "":
		p.Helm, p.Release = true, instance
	case instance != "" && labels["app.kubernetes.io/name"] == "meerkat":
		// The chart puts its selector labels on the pod, and only those: the
		// name and the release.
		p.Helm, p.Release = true, instance
	}
	p.Workload = workloadOf(podName, labels)
	return p
}

// workloadOf is the Deployment (or StatefulSet) a pod's name was made from:
// <deployment>-<template hash>-<suffix>, <statefulset>-<ordinal>.
func workloadOf(pod string, labels map[string]string) string {
	if hash := labels["pod-template-hash"]; hash != "" {
		if i := strings.LastIndex(pod, "-"); i > 0 {
			if base, ok := strings.CutSuffix(pod[:i], "-"+hash); ok {
				return base
			}
		}
	}
	if labels["statefulset.kubernetes.io/pod-name"] == pod {
		if i := strings.LastIndex(pod, "-"); i > 0 {
			return pod[:i]
		}
	}
	return ""
}

// dockerPlatform reads a container's labels: Swarm's, compose's, or none.
func dockerPlatform(labels map[string]string, containerName string) Platform {
	if s := labels["com.docker.swarm.service.name"]; s != "" {
		return Platform{Name: "swarm", Workload: s, Stack: labels["com.docker.stack.namespace"]}
	}
	if s := labels["com.docker.compose.service"]; s != "" {
		p := Platform{Name: "compose", Workload: s, Stack: labels["com.docker.compose.project"],
			Dir: labels["com.docker.compose.project.working_dir"], Container: strings.TrimPrefix(containerName, "/")}
		for _, f := range strings.Split(labels["com.docker.compose.project.config_files"], ",") {
			if f = strings.TrimSpace(f); f != "" {
				p.Files = append(p.Files, f)
			}
		}
		return p
	}
	return Platform{Name: "docker", Workload: strings.TrimPrefix(containerName, "/")}
}

// inContainer says the process runs in a container: Docker's marker, or
// Podman's.
func inContainer() bool {
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return false
}
