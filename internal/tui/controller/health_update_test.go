package controller

import (
	"envctl/internal/reporting"
	"envctl/internal/tui/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestHandleHealthStatusMsg_WCUpdate tests that WC health updates are properly processed
func TestHandleHealthStatusMsg_WCUpdate(t *testing.T) {
	// Create a test model with both MC and WC
	testModel := &model.Model{
		ManagementClusterName: "test-mc",
		WorkloadClusterName:   "test-wc",
		MCHealth:              model.ClusterHealthInfo{IsLoading: true},
		WCHealth:              model.ClusterHealthInfo{IsLoading: true},
		DebugMode:             true, // Enable debug mode to see logs
	}

	// Test 1: MC health update
	t.Run("MC_HealthUpdate", func(t *testing.T) {
		msg := reporting.HealthStatusMsg{
			Update: reporting.HealthStatusUpdate{
				Timestamp:        time.Now(),
				ContextName:      "teleport.giantswarm.io-test-mc",
				ClusterShortName: "test-mc",
				IsMC:             true,
				IsHealthy:        true,
				ReadyNodes:       3,
				TotalNodes:       3,
				Error:            nil,
			},
		}

		updatedModel, _ := handleHealthStatusMsg(testModel, msg)

		// Verify MC health was updated
		assert.False(t, updatedModel.MCHealth.IsLoading, "MC should not be loading")
		assert.Equal(t, 3, updatedModel.MCHealth.ReadyNodes)
		assert.Equal(t, 3, updatedModel.MCHealth.TotalNodes)
		assert.Nil(t, updatedModel.MCHealth.StatusError)
	})

	// Test 2: WC health update
	t.Run("WC_HealthUpdate", func(t *testing.T) {
		msg := reporting.HealthStatusMsg{
			Update: reporting.HealthStatusUpdate{
				Timestamp:        time.Now(),
				ContextName:      "teleport.giantswarm.io-test-mc-test-wc",
				ClusterShortName: "test-wc",
				IsMC:             false,
				IsHealthy:        true,
				ReadyNodes:       5,
				TotalNodes:       5,
				Error:            nil,
			},
		}

		updatedModel, _ := handleHealthStatusMsg(testModel, msg)

		// Verify WC health was updated
		assert.False(t, updatedModel.WCHealth.IsLoading, "WC should not be loading")
		assert.Equal(t, 5, updatedModel.WCHealth.ReadyNodes)
		assert.Equal(t, 5, updatedModel.WCHealth.TotalNodes)
		assert.Nil(t, updatedModel.WCHealth.StatusError)
	})

	// Test 3: WC health update with error
	t.Run("WC_HealthUpdate_Error", func(t *testing.T) {
		testError := assert.AnError
		msg := reporting.HealthStatusMsg{
			Update: reporting.HealthStatusUpdate{
				Timestamp:        time.Now(),
				ContextName:      "teleport.giantswarm.io-test-mc-test-wc",
				ClusterShortName: "test-wc",
				IsMC:             false,
				IsHealthy:        false,
				ReadyNodes:       0,
				TotalNodes:       0,
				Error:            testError,
			},
		}

		updatedModel, _ := handleHealthStatusMsg(testModel, msg)

		// Verify WC health error was recorded
		assert.False(t, updatedModel.WCHealth.IsLoading, "WC should not be loading")
		assert.NotNil(t, updatedModel.WCHealth.StatusError)
		assert.Equal(t, testError, updatedModel.WCHealth.StatusError)
	})
}

// TestHandleHealthStatusMsg_EmptyClusterName tests what happens if cluster name is empty
func TestHandleHealthStatusMsg_EmptyClusterName(t *testing.T) {
	testModel := &model.Model{
		ManagementClusterName: "test-mc",
		WorkloadClusterName:   "test-wc",
		MCHealth:              model.ClusterHealthInfo{IsLoading: true},
		WCHealth:              model.ClusterHealthInfo{IsLoading: true},
	}

	// Send health update with empty cluster name
	msg := reporting.HealthStatusMsg{
		Update: reporting.HealthStatusUpdate{
			Timestamp:        time.Now(),
			ContextName:      "teleport.giantswarm.io-test-mc-test-wc",
			ClusterShortName: "", // Empty cluster name
			IsMC:             false,
			IsHealthy:        true,
			ReadyNodes:       5,
			TotalNodes:       5,
			Error:            nil,
		},
	}

	// This should still process but may not update the correct health struct
	updatedModel, _ := handleHealthStatusMsg(testModel, msg)

	// The handler will pass empty cluster name to handleNodeStatusMsg
	// which should handle it gracefully
	assert.NotNil(t, updatedModel)
}
