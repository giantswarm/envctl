package tui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// This is a helper function that can be called to manually verify the fixes for the log panel height issue
func TestLogPanelHeight(t *testing.T) {
	// Test 1: Simple test to verify that lipgloss.Place correctly fills vertical space
	fmt.Println("=== Test 1: lipgloss.Place with fixed height ===")
	containerHeight := 10
	content := "Line 1\nLine 2\nLine 3" // Only 3 lines
	
	// Using Place should force it to fill 10 lines
	placedContent := lipgloss.Place(
		30, // width
		containerHeight,
		lipgloss.Left,
		lipgloss.Top,
		content)
	
	fmt.Printf("Content height: %d, Placed height: %d\n", 
		lipgloss.Height(content), 
		lipgloss.Height(placedContent))
	fmt.Println(placedContent)
	
	// Test 2: When using Style.Height() versus JoinVertical behavior
	fmt.Println("\n=== Test 2: Style.Height() versus JoinVertical behavior ===")
	// Create a styled box with fixed height
	styledBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Height(containerHeight - 4). // Account for border and padding
		Width(28).
		Render(content)
	
	fmt.Printf("Content height: %d, Styled height: %d\n", 
		lipgloss.Height(content), 
		lipgloss.Height(styledBox))
	fmt.Println(styledBox)
	
	// Test 3: Fix approach - force height with extra padding
	fmt.Println("\n=== Test 3: Fix approach - force height with extra padding ===")
	styledBox2 := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(28).
		Render(content)
	
	actualHeight := lipgloss.Height(styledBox2)
	targetHeight := containerHeight
	
	var finalRendered string
	if actualHeight < targetHeight {
		// Add padding to force the correct height
		heightDiff := targetHeight - actualHeight
		paddingStr := ""
		for i := 0; i < heightDiff; i++ {
			paddingStr += "\n"
		}
		finalRendered = lipgloss.JoinVertical(lipgloss.Left, styledBox2, paddingStr)
	} else {
		finalRendered = styledBox2
	}
	
	fmt.Printf("Original height: %d, Padded height: %d\n", 
		lipgloss.Height(styledBox2), 
		lipgloss.Height(finalRendered))
	fmt.Println(finalRendered)

	// Test 4: What's happening in our actual code
	fmt.Println("\n=== Test 4: Viewport in panel ===")
	// Simulate what happens with the mainLogViewport
	viewportContent := "Log line 1\nLog line 2\nLog line 3\nLog line 4"
	titleView := lipgloss.NewStyle().Bold(true).Render("Combined Activity Log")
	
	// Join the title and content
	joinedContent := lipgloss.JoinVertical(lipgloss.Left, titleView, viewportContent)
	
	// Render in a panel
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(30)
	
	panelRendered := panelStyle.Render(joinedContent)
	targetLogHeight := 15
	
	fmt.Printf("Panel height: %d, Target height: %d\n", 
		lipgloss.Height(panelRendered), 
		targetLogHeight)
	
	// Force correct height with padding
	actualPanelHeight := lipgloss.Height(panelRendered)
	if actualPanelHeight < targetLogHeight {
		heightDiff := targetLogHeight - actualPanelHeight
		paddingStr := ""
		for i := 0; i < heightDiff; i++ {
			paddingStr += "\n"
		}
		panelRendered = lipgloss.JoinVertical(lipgloss.Left, panelRendered, paddingStr)
	}
	
	fmt.Printf("Original panel height: %d, Padded panel height: %d\n", 
		actualPanelHeight, 
		lipgloss.Height(panelRendered))
} 