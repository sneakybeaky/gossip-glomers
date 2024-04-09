package main

import "sync"

type values struct {
	mx sync.RWMutex
	vv map[float64]bool
}

func newValues() *values {
	return &values{
		vv: make(map[float64]bool),
	}
}

func (m *values) Store(values ...float64) {
	m.mx.Lock()
	defer m.mx.Unlock()

	for _, v := range values {
		m.vv[v] = true
	}

}

func (m *values) Values() []float64 {
	m.mx.RLock()
	defer m.mx.RUnlock()

	var messages = make([]float64, len(m.vv))
	i := 0
	for k := range m.vv {
		messages[i] = k
		i++
	}

	return messages
}

func (m *values) Seen(message float64) bool {
	m.mx.RLock()
	defer m.mx.RUnlock()
	seen, _ := m.vv[message]
	return seen
}
