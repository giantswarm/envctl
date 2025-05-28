// Package color provides terminal color detection and theming for envctl.
//
// This package handles the complexity of terminal color support detection
// and provides consistent color theming across the application. It ensures
// that envctl displays correctly in various terminal environments.
//
// # Core Functionality
//
// The package provides:
//   - Terminal color capability detection
//   - Adaptive color profiles (TrueColor, 256 colors, 16 colors, no color)
//   - Dark and light theme support
//   - Consistent color palette for UI elements
//
// # Color Detection
//
// The package automatically detects terminal capabilities by checking:
//   - COLORTERM environment variable for TrueColor support
//   - TERM environment variable for color capabilities
//   - NO_COLOR environment variable to disable colors
//   - Terminal type specific features
//
// # Theme System
//
// Colors are organized into semantic categories:
//   - Primary: Main brand colors
//   - Success: Positive states (running, healthy)
//   - Warning: Caution states (starting, degraded)
//   - Error: Failure states (stopped, unhealthy)
//   - Info: Informational elements
//   - Muted: De-emphasized text
//
// # Usage Example
//
//	// Get the current color profile
//	profile := color.GetProfile()
//
//	// Create styles based on profile
//	successStyle := lipgloss.NewStyle().
//	    Foreground(profile.Success)
//
//	errorStyle := lipgloss.NewStyle().
//	    Foreground(profile.Error).
//	    Bold(true)
//
//	// Apply styles
//	fmt.Println(successStyle.Render("✓ Service running"))
//	fmt.Println(errorStyle.Render("✗ Service failed"))
//
// # Adaptive Rendering
//
// The package automatically adapts colors based on terminal capabilities:
//   - TrueColor: Full RGB color support
//   - 256 colors: Closest match from 256 color palette
//   - 16 colors: Basic ANSI colors
//   - No color: Plain text output
//
// # Environment Variables
//
// Respected environment variables:
//   - NO_COLOR: Disable all color output
//   - COLORTERM: Indicates TrueColor support
//   - TERM: Terminal type for capability detection
//   - ENVCTL_THEME: Force dark or light theme
//
// # Thread Safety
//
// All color detection and profile functions are thread-safe and can
// be called concurrently.
package color
