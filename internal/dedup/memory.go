package dedup

import "sync"

type Memory struct{ m sync.Map }

func NewMemory() *Memory { return &Memory{} }

func (d *Memory) Seen(key string) bool {
	_, ok := d.m.LoadOrStore(key, struct{}{})
	return ok
}
