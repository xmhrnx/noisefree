package vmic

import "strings"

// endpoints lists PulseAudio/PipeWire sinks (outputs) or sources (inputs).
func endpoints(output bool) ([]Endpoint, error) {
	kind, getDefault := "sources", "get-default-source"
	if output {
		kind, getDefault = "sinks", "get-default-sink"
	}
	def, err := pactl(getDefault)
	if err != nil {
		return nil, err
	}
	out, err := pactl("list", kind)
	if err != nil {
		return nil, err
	}
	var eps []Endpoint
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Name: "):
			id := strings.TrimPrefix(line, "Name: ")
			eps = append(eps, Endpoint{ID: id, Name: id, Default: id == def})
		case strings.HasPrefix(line, "Description: ") && len(eps) > 0:
			eps[len(eps)-1].Name = strings.TrimPrefix(line, "Description: ")
		}
	}
	return eps, nil
}

func setDefault(output bool, e Endpoint) error {
	cmd := "set-default-source"
	if output {
		cmd = "set-default-sink"
	}
	_, err := pactl(cmd, e.ID)
	return err
}
