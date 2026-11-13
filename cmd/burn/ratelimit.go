package main

import (
	"sync"
	"time"
)

// RateLimiter 是每 IP 的固定窗口计数器。
// limit <= 0 表示不限制——写成 `count >= limit` 会让 0 变成「全部拒绝」，这里先短路。
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*windowEntry
}

type windowEntry struct {
	count int
	start time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]*windowEntry),
	}
}

func (r *RateLimiter) Allow(ip string) bool {
	if r == nil || r.limit <= 0 {
		return true
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[ip]
	if !ok || now.Sub(e.start) >= r.window {
		r.entries[ip] = &windowEntry{count: 1, start: now}
		return true
	}

	if e.count >= r.limit {
		return false
	}
	e.count++
	return true
}
