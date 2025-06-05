package view

import "time"

const (
	// healthUpdateInterval defines how often cluster health information (node status) is refreshed.
	healthUpdateInterval = 30 * time.Second
)
