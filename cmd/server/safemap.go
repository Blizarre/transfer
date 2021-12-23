package main

import (
	"io"
	"sync"
)

type SafeMap struct {
	l sync.RWMutex
	m map[string]chan io.Writer
}

func NewSafeMap() SafeMap {
	return SafeMap{m: make(map[string]chan io.Writer)}
}

func (s *SafeMap) Read(id string) (chan io.Writer, bool) {
	s.l.Lock()
	defer s.l.Unlock()
	v, ok := s.m[id]
	return v, ok
}

func (s *SafeMap) Add(id string, channel chan io.Writer) {
	s.l.Lock()
	defer s.l.Unlock()
	s.m[id] = channel
}
