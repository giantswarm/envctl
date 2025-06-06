package components_test

import (
	"envctl/internal/tui/components"
	"envctl/internal/tui/design"
	"fmt"
)

// ExamplePanel demonstrates how to create and use a panel component
func ExamplePanel() {
	// Create a basic panel
	panel := components.NewPanel("Service Status").
		WithContent("Service is running smoothly\nNo errors detected").
		WithDimensions(40, 10).
		WithType(components.PanelTypeSuccess).
		WithIcon(design.SafeIcon(design.IconCheck))

	// Render the panel
	output := panel.Render()
	fmt.Println(len(output) > 0) // Check that output was generated
	// Output: true
}

// ExampleStatusIndicator demonstrates status indicator usage
func ExampleStatusIndicator() {
	// Create different status indicators
	running := components.NewStatusIndicator(components.StatusTypeRunning)
	failed := components.NewStatusIndicator(components.StatusTypeFailed).
		WithText("Connection Failed")

	// Icon-only indicator
	healthy := components.NewStatusIndicator(components.StatusTypeHealthy).
		IconOnly()

	fmt.Println(len(running.Render()) > 0)
	fmt.Println(len(failed.Render()) > 0)
	fmt.Println(len(healthy.Render()) > 0)
	// Output:
	// true
	// true
	// true
}

// ExampleHeader demonstrates header component usage
func ExampleHeader() {
	// Create a header with subtitle
	header := components.NewHeader("My Application").
		WithSubtitle("v1.0.0 - Press h for help").
		WithWidth(80)

	output := header.Render()
	fmt.Println(len(output) > 0)
	// Output: true
}

// ExampleStatusBar demonstrates status bar usage
func ExampleStatusBar() {
	// Create a status bar with a message
	statusBar := components.NewStatusBar(80).
		WithLeftText("Connected").
		WithRightText("3 services running")

	output := statusBar.Render()
	fmt.Println(len(output) > 0)
	// Output: true
}

// ExampleLayout demonstrates layout management
func ExampleLayout() {
	// Create a layout manager
	layout := components.NewLayout(100, 50)

	// Split the screen
	topHeight, bottomHeight := layout.SplitHorizontal(0.3) // 30% top, 70% bottom
	leftWidth, rightWidth := layout.SplitVertical(0.4)     // 40% left, 60% right

	fmt.Println(topHeight > 0)
	fmt.Println(bottomHeight > 0)
	fmt.Println(leftWidth > 0)
	fmt.Println(rightWidth > 0)
	// Output:
	// true
	// true
	// true
	// true
}
