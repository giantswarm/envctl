package tui

// -------------------- Cluster / kube-context messages --------------------

type kubeContextResultMsg struct {
    context string
    err     error
}

type nodeStatusMsg struct {
    clusterShortName string // e.g. "myinstallation" or "myinstallation-mywc"
    forMC            bool
    readyNodes       int
    totalNodes       int
    err              error
    debugInfo        string
}

type requestClusterHealthUpdate struct{}