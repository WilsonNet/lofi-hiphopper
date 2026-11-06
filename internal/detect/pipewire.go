package detect

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Linux sound servers describe audio activity as streams rather than
// processes. These parsers are platform-neutral so they can be tested
// anywhere; the Linux probe feeds them pw-dump or pactl output.

const (
	pwNodeType      = "PipeWire:Interface:Node"
	pwClientType    = "PipeWire:Interface:Client"
	pwOutputStream  = "Stream/Output/Audio"
	pwInputStream   = "Stream/Input/Audio"
	pwRunningState  = "running"
	pulseClientAPI  = "pipewire-pulse"
	monitorSuffix   = ".monitor"
	propCaptureSink = "stream.capture.sink"
	propMonitor     = "stream.monitor"
)

type pwObject struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Info *struct {
		State string         `json:"state"`
		Props map[string]any `json:"props"`
	} `json:"info"`
}

// ParsePipeWireDump turns `pw-dump` JSON into one Process per audio stream
// node. A stream counts as active only while its node state is "running";
// paused or corked streams go idle and are ignored. Captures of a sink
// monitor (visualizers, recorders of desktop audio) are not microphone use and
// are skipped.
func ParsePipeWireDump(data []byte) ([]Process, error) {
	var objects []pwObject
	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, fmt.Errorf("decode pw-dump: %w", err)
	}
	clients := make(map[int]map[string]any)
	for _, object := range objects {
		if object.Type == pwClientType && object.Info != nil {
			clients[object.ID] = object.Info.Props
		}
	}

	var processes []Process
	for _, object := range objects {
		if object.Type != pwNodeType || object.Info == nil {
			continue
		}
		props := object.Info.Props
		class := propString(props, "media.class")
		if class != pwOutputStream && class != pwInputStream {
			continue
		}
		if object.Info.State != pwRunningState {
			continue
		}
		if class == pwInputStream && isMonitorCapture(props) {
			continue
		}
		client := clients[propInt(props, "client.id")]
		process := Process{
			PID:      streamPID(props, client),
			Name:     firstString(props, client, "application.name", "node.name"),
			BundleID: firstString(props, client, "application.id", "pipewire.access.portal.app_id", "pipewire.access.flatpak.app_id"),
			Path:     firstString(props, client, "application.process.binary"),
			Role:     firstString(props, client, "media.role"),
			Input:    class == pwInputStream,
			Output:   class == pwOutputStream,
		}
		processes = append(processes, process)
	}
	return processes, nil
}

type pulseStream struct {
	Corked     bool           `json:"corked"`
	Properties map[string]any `json:"properties"`
}

// ParsePulseStreams turns `pactl -f json list sink-inputs` and
// `pactl -f json list source-outputs` output into processes. Corked streams
// are paused and do not count.
func ParsePulseStreams(sinkInputs, sourceOutputs []byte) ([]Process, error) {
	var processes []Process
	for _, part := range []struct {
		data  []byte
		input bool
		name  string
	}{
		{sinkInputs, false, "sink-inputs"},
		{sourceOutputs, true, "source-outputs"},
	} {
		if len(strings.TrimSpace(string(part.data))) == 0 {
			continue
		}
		var streams []pulseStream
		if err := json.Unmarshal(part.data, &streams); err != nil {
			return nil, fmt.Errorf("decode pactl %s: %w", part.name, err)
		}
		for _, stream := range streams {
			props := stream.Properties
			if stream.Corked || propBool(props, "pulse.corked") {
				continue
			}
			if part.input && isMonitorCapture(props) {
				continue
			}
			processes = append(processes, Process{
				PID:      propInt(props, "application.process.id"),
				Name:     firstString(props, nil, "application.name", "media.name"),
				BundleID: firstString(props, nil, "application.id", "pipewire.access.portal.app_id"),
				Path:     firstString(props, nil, "application.process.binary"),
				Role:     propString(props, "media.role"),
				Input:    part.input,
				Output:   !part.input,
			})
		}
	}
	return processes, nil
}

// streamPID prefers the PID the application reported on the node, then the
// client's reported PID, then the socket credential PID for native PipeWire
// clients. pipewire-pulse clients share the server's socket PID, so that
// fallback would be wrong for them.
func streamPID(node, client map[string]any) int {
	if pid := propInt(node, "application.process.id"); pid > 0 {
		return pid
	}
	if pid := propInt(client, "application.process.id"); pid > 0 {
		return pid
	}
	if propString(client, "client.api") != pulseClientAPI {
		return propInt(client, "pipewire.sec.pid")
	}
	return 0
}

func isMonitorCapture(props map[string]any) bool {
	if propBool(props, propCaptureSink) || propBool(props, propMonitor) {
		return true
	}
	for _, key := range []string{"target.object", "node.target", "pulse.source.name"} {
		if strings.HasSuffix(propString(props, key), monitorSuffix) {
			return true
		}
	}
	return false
}

func firstString(primary, secondary map[string]any, keys ...string) string {
	for _, key := range keys {
		for _, props := range []map[string]any{primary, secondary} {
			if value := propString(props, key); value != "" {
				return value
			}
		}
	}
	return ""
}

func propString(props map[string]any, key string) string {
	switch value := props[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}

func propInt(props map[string]any, key string) int {
	switch value := props[key].(type) {
	case float64:
		return int(value)
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}
		return number
	default:
		return 0
	}
}

func propBool(props map[string]any, key string) bool {
	switch value := props[key].(type) {
	case bool:
		return value
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
		return parsed
	default:
		return false
	}
}
