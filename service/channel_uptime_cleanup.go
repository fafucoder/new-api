package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	channelUptimeRetention   = 7 * 24 * time.Hour
	channelUptimeCleanupTick = 24 * time.Hour
)

var autoUptimeCleanupOnce sync.Once

// AutomaticallyCleanupUptimeRecords removes channel_uptime_records older than
// the retention window once a day. Runs only on the master node, matching the
// existing AutomaticallyTestChannels pattern.
func AutomaticallyCleanupUptimeRecords() {
	if !common.IsMasterNode {
		return
	}
	autoUptimeCleanupOnce.Do(func() {
		runUptimeCleanup()
		ticker := time.NewTicker(channelUptimeCleanupTick)
		go func() {
			for range ticker.C {
				runUptimeCleanup()
			}
		}()
	})
}

func runUptimeCleanup() {
	if err := model.CleanupExpiredUptimeRecords(channelUptimeRetention); err != nil {
		common.SysError("cleanup expired channel uptime records failed: " + err.Error())
		return
	}
	common.SysLog("channel uptime records older than 7 days cleaned up")
}
