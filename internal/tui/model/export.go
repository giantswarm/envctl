package model

// InputStep represents which field the new-connection input wizard is currently on.
// This was previously a mirror of an internal enum.
type InputStep int

const (
	McInputStep InputStep = iota
	WcInputStep
)

// ViewModel interface removed to avoid conflicts with Model struct's exported fields.
// View components will access Model fields directly.
