package tui

import "envctl/internal/utils"

// -------------------- New connection flow messages --------------------

type startNewConnectionInputMsg struct{}

type submitNewConnectionMsg struct {
    mc string
    wc string
}

type cancelNewConnectionInputMsg struct{}

type mcNameEnteredMsg struct {
    mc string
}

type kubeLoginResultMsg struct {
    clusterName        string
    isMC               bool
    desiredWcShortName string
    loginStdout        string
    loginStderr        string
    err                error
}

type contextSwitchAndReinitializeResultMsg struct {
    switchedContext string
    desiredMcName   string
    desiredWcName   string
    diagnosticLog   string
    err             error
}

type kubeContextSwitchedMsg struct {
    TargetContext string
    err           error
    DebugInfo     string // Additional debug information for context switching
}

type clusterListResultMsg struct {
    info *utils.ClusterInfo
    err  error
} 