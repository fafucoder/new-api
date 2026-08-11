/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package maas

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateSeedanceTokensKeepsLegacyMultiplierSeparate(t *testing.T) {
	raw := estimateSeedanceRawTokens("720p", "16:9", 5)
	withReference := estimateSeedanceTokens("720p", "16:9", 5, true)
	withoutReference := estimateSeedanceTokens("720p", "16:9", 5, false)

	assert.Equal(t, 108000, raw)
	assert.Equal(t, raw, withReference)
	assert.Greater(t, withoutReference, raw)
}

func TestAdjustBillingOnCompleteUsesRawTokensForDynamicPricing(t *testing.T) {
	snapshotBytes, err := encodeRequestSnapshot(requestSnapshot{
		Resolution:    "720p",
		Ratio:         "16:9",
		Duration:      5,
		HasVideoInput: false,
	})
	require.NoError(t, err)

	adaptor := &TaskAdaptor{}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}
	task := &model.Task{
		PrivateData: model.TaskPrivateData{
			RequestSnapshot: snapshotBytes,
			BillingContext: &model.TaskBillingContext{
				TieredBillingSnapshot: &billingexpr.BillingSnapshot{},
			},
		},
	}

	assert.Zero(t, adaptor.AdjustBillingOnComplete(task, taskResult))
	assert.Equal(t, 108000, taskResult.TotalTokens)

	legacyResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}
	task.PrivateData.BillingContext.TieredBillingSnapshot = nil
	assert.Zero(t, adaptor.AdjustBillingOnComplete(task, legacyResult))
	assert.Greater(t, legacyResult.TotalTokens, taskResult.TotalTokens)
}
