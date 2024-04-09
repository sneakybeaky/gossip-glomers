package main

type topology struct {
	t map[string][]string
}

func newTopology(t map[string][]string) *topology {
	return &topology{t: t}
}

// AllBut returns a slice of strings containing all elements except the one with the given name.
func (t topology) AllBut(n string) []string {

	r := make([]string, len(t.t)-1)

	i := 0
	for address := range t.t {
		if address == n {
			continue //
		}
		r[i] = address
		i++
	}

	return r
}
