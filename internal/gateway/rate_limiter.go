package gateway

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter 是基于内存滑动窗口的调用频率限制器。
// 按 "agent_id:skill_name:caller" 维度独立计数。
type RateLimiter struct {
	mu      sync.Mutex
	windows map[string]*slidingWindow
}

type slidingWindow struct {
	timestamps []time.Time
	max        int
	window     time.Duration
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		windows: make(map[string]*slidingWindow),
	}
}

// Allow 检查是否允许本次调用。
// 如果 max <= 0，表示不限流，直接放行。
// callerKey 通常是 "user:{userID}" 或 "agent:{agentID}"。
func (r *RateLimiter) Allow(agentID, skillName, callerKey string, max int, window time.Duration) bool {
	if max <= 0 {
		return true
	}

	key := fmt.Sprintf("%s:%s:%s", agentID, skillName, callerKey)

	r.mu.Lock()
	defer r.mu.Unlock()

	w, ok := r.windows[key]
	if !ok {
		w = &slidingWindow{max: max, window: window}
		r.windows[key] = w
	}

	now := time.Now()
	cutoff := now.Add(-window)

	// 清理过期的时间戳
	valid := w.timestamps[:0]
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	w.timestamps = valid

	// 检查是否超过限制
	if len(w.timestamps) >= max {
		return false
	}

	w.timestamps = append(w.timestamps, now)
	return true
}

// Cleanup 清理过期的滑动窗口条目，防止内存泄漏。
// 应由后台 goroutine 定期调用。
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for key, w := range r.windows {
		cutoff := now.Add(-w.window)
		valid := w.timestamps[:0]
		for _, ts := range w.timestamps {
			if ts.After(cutoff) {
				valid = append(valid, ts)
			}
		}
		if len(valid) == 0 {
			delete(r.windows, key)
		} else {
			w.timestamps = valid
		}
	}
}

// StartCleanup 启动后台清理协程，每隔 interval 清理一次过期条目。
// stopCh 关闭时停止清理。
func (r *RateLimiter) StartCleanup(interval time.Duration, stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				r.Cleanup()
			}
		}
	}()
}
