package rules

import (
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"regexp"
	"strconv"
	"strings"
)

// Correlate adds independent current observations to journal-based hypotheses.
// It deliberately keeps causal confidence probable because journal lines may be historical.
func Correlate(findings []types.Finding, obs []types.Observation) []types.Finding {
	var sockets []types.Socket
	processes := map[int]types.Process{}
	services := map[string]types.Service{}
	for _, o := range obs {
		switch data := o.Data.(type) {
		case types.Network:
			sockets = append(sockets, data.Sockets...)
		case []types.Process:
			for _, p := range data {
				processes[p.PID] = p
			}
		case types.Service:
			services[data.Name] = data
		}
	}
	portPattern := regexp.MustCompile(`(?::|port\s+)([0-9]{1,5})(?:\b)`)
	for i := range findings {
		f := &findings[i]
		if !strings.HasPrefix(f.ID, "service.") {
			continue
		}
		name := strings.SplitN(f.ID, ":", 2)
		if len(name) != 2 {
			continue
		}
		s, ok := services[name[1]]
		if !ok {
			continue
		}
		if p, ok := processes[s.PID]; ok {
			f.Evidence = append(f.Evidence, fmt.Sprintf("Current process PID %d (%s), RSS %d bytes, state %s", p.PID, p.Name, p.RSS, p.State))
		}
		if !strings.HasPrefix(f.ID, "service.bind:") {
			continue
		}
		seen := map[int]bool{}
		for _, line := range s.Logs {
			low := strings.ToLower(line)
			if !strings.Contains(low, "address already in use") && !strings.Contains(low, "eaddrinuse") {
				continue
			}
			for _, m := range portPattern.FindAllStringSubmatch(low, -1) {
				port, _ := strconv.Atoi(m[1])
				for _, socket := range sockets {
					if socket.Port == port && !seen[port] {
						seen[port] = true
						f.Evidence = append(f.Evidence, fmt.Sprintf("Current %s socket %s:%d inode %s", socket.Protocol, socket.Address, socket.Port, socket.Inode))
						f.Explanation = "A service bind-conflict message names a port that is also currently occupied. Check socket ownership and configuration to establish causality; the journal entry may be historical."
					}
				}
			}
		}
	}
	return findings
}
