package disks

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nos/backend/nosd/pkg/shell"
)

type lsblkJSON struct {
	Blockdevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string        `json:"name"`
	KName      string        `json:"kname"`
	Path       string        `json:"path"`
	Size       any           `json:"size"`
	Rota       *bool         `json:"rota"`
	Type       string        `json:"type"`
	Tran       string        `json:"tran"`
	Vendor     string        `json:"vendor"`
	Model      string        `json:"model"`
	Serial     string        `json:"serial"`
	Mountpoint *string       `json:"mountpoint"`
	FSType     string        `json:"fstype"`
	Children   []lsblkDevice `json:"children"`
}

func ParseSizeToBytes(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

func Collect(ctx context.Context) ([]Disk, error) {
	args := []string{"-J", "-O", "-o", "NAME,KNAME,PATH,SIZE,ROTA,TYPE,TRAN,VENDOR,MODEL,SERIAL,MOUNTPOINT,FSTYPE"}
	res, err := shell.Run(ctx, 5*time.Second, "lsblk", args...)
	if err != nil {
		return nil, err
	}
	var tree lsblkJSON
	if err := json.Unmarshal(res.Stdout, &tree); err != nil {
		return nil, err
	}
	out := []Disk{}
	var walk func(d lsblkDevice)
	walk = func(d lsblkDevice) {
		if d.Type == "disk" || d.Type == "part" {
			out = append(out, Disk{
				Name:       d.Name,
				KName:      d.KName,
				Path:       d.Path,
				SizeBytes:  ParseSizeToBytes(d.Size),
				Rota:       d.Rota,
				Type:       d.Type,
				Tran:       d.Tran,
				Vendor:     d.Vendor,
				Model:      d.Model,
				Serial:     d.Serial,
				Mountpoint: d.Mountpoint,
				FSType:     d.FSType,
			})
		}
		for _, c := range d.Children {
			walk(c)
		}
	}
	for _, d := range tree.Blockdevices {
		walk(d)
	}
	return out, nil
}

func SmartSummaryFor(ctx context.Context, devicePath string) *SmartSummary {
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil
	}
	res, err := shell.Run(ctx, 3*time.Second, "smartctl", "-H", "-A", devicePath, "-j")
	if err != nil {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(res.Stdout, &parsed); err != nil {
		return nil
	}
	var healthy *bool
	if s, ok := parsed["smart_status"].(map[string]any); ok {
		if p, ok := s["passed"].(bool); ok {
			healthy = &p
		}
	}
	var temp *int
	if t, ok := parsed["temperature"].(map[string]any); ok {
		if c, ok := t["current"].(float64); ok {
			v := int(c)
			temp = &v
		}
	}
	var poh *int
	var reallocated *int
	var mediaErr *int
	if a, ok := parsed["power_on_time"].(map[string]any); ok {
		if h, ok := a["hours"].(float64); ok {
			v := int(h)
			poh = &v
		}
	}
	// try vendor tables for extended info
	if table, ok := parsed["ata_smart_attributes"].(map[string]any); ok {
		if jv, ok2 := table["table"].([]any); ok2 {
			for _, row := range jv {
				if m, ok3 := row.(map[string]any); ok3 {
					if name, ok4 := m["name"].(string); ok4 {
						ln := strings.ToLower(name)
						if strings.Contains(ln, "reallocated") {
							if raw, ok5 := m["raw"].(map[string]any); ok5 {
								if val, ok6 := raw["value"].(float64); ok6 {
									x := int(val)
									reallocated = &x
								}
							}
						}
					}
				}
			}
		}
	}
	if nvme, ok := parsed["nvme_smart_health_information_log"].(map[string]any); ok {
		if me, ok2 := nvme["media_errors"].(float64); ok2 {
			v := int(me)
			mediaErr = &v
		}
		if t, ok2 := nvme["temperature"].(float64); ok2 && temp == nil {
			vv := int(t)
			temp = &vv
		}
	}
	if healthy == nil && temp == nil && poh == nil {
		return nil
	}
	return &SmartSummary{Healthy: healthy, TempCelsius: temp, PowerOnHours: poh, Reallocated: reallocated, MediaErrors: mediaErr}
}
