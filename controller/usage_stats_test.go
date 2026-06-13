package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveWindow(t *testing.T) {
	// 已知窗口原样返回
	w, cfg := resolveWindow("5m")
	assert.Equal(t, "5m", w)
	assert.Equal(t, int64(30), cfg.bucketSize)
	assert.Equal(t, int64(10), cfg.bucketNum)

	w2, cfg2 := resolveWindow("1d")
	assert.Equal(t, "1d", w2)
	assert.Equal(t, int64(3600), cfg2.bucketSize)
	assert.Equal(t, int64(24), cfg2.bucketNum)

	// 非法/缺省回退 1h
	w3, cfg3 := resolveWindow("bogus")
	assert.Equal(t, "1h", w3)
	assert.Equal(t, int64(300), cfg3.bucketSize)
	assert.Equal(t, int64(12), cfg3.bucketNum)
}

func TestComputeRange_AlignedAndLength(t *testing.T) {
	_, cfg := resolveWindow("1h")
	start, end := computeRange(cfg)
	// start 对齐到 bucketSize 网格
	assert.Equal(t, int64(0), start%cfg.bucketSize)
	// 窗口跨度 = (桶数-1) * bucketSize
	assert.Equal(t, (cfg.bucketNum-1)*cfg.bucketSize, ((end/cfg.bucketSize)*cfg.bucketSize)-start)
}
