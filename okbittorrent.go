package main

import "sync"

type torrentInfo struct {
}

type torrentPeers []torrentPeer

type torrentPeer struct {
}

var torrentTrackers []string

var torrentNewPeers chan torrentPeer

var torrentNewTrackers chan string

type SafeCounter struct {
	mu sync.Mutex
	v  map[string]int
}

func (c *SafeCounter) Inc(key string) {
	c.mu.Lock()
	c.v[key]++
	c.mu.Unlock()
}

func (c *SafeCounter) Value(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.v[key]
}

func (c *SafeCounter) Dec(key string) {
	c.mu.Lock()
	c.v[key]--
	c.mu.Unlock()
}

var connectionCounter SafeCounter

func main() {
}
