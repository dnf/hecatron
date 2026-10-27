package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ParseSlots is -j: an integer, a percentage of the CPUs (1%-200%, default
// 50%), "pcores" (the performance cores less one) or "unlimited" (0, as is
// "0"). Its errors are run-parallel.sh's.
func ParseSlots(value string) (int, error) {
	if value == "" {
		value = "50%"
	}
	switch {
	case value == "unlimited" || value == "0":
		return 0, nil
	case value == "pcores":
		return max(performanceCores()-1, 1), nil
	case strings.HasSuffix(value, "%"):
		percent, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
		if err != nil || percent < 1 || percent > 200 {
			return 0, fmt.Errorf("-j percentage must be 1%%-200%% (got '%s')", value)
		}
		cpus := runtime.NumCPU()
		if percent == 100 {
			return cpus, nil
		}
		return max((cpus*percent+50)/100, 1), nil
	}
	if strings.Trim(value, "0123456789") != "" {
		return 0, fmt.Errorf("-j accepts integer, percentage (1%%-200%%), 'pcores', or 'unlimited'")
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("-j accepts integer, percentage (1%%-200%%), 'pcores', or 'unlimited'")
	}
	return n, nil
}

// performanceCores counts the fastest cores: Apple Silicon's performance
// cores, Linux's CPUs at the highest maximum frequency, else every CPU.
func performanceCores() int {
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("sysctl", "-n", "hw.perflevel0.logicalcpu").Output(); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n > 0 {
				return n
			}
		}
	}
	files, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/cpuinfo_max_freq")
	counts := map[int]int{}
	highest := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if f, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			counts[f]++
			highest = max(highest, f)
		}
	}
	if highest > 0 {
		return counts[highest]
	}
	return runtime.NumCPU()
}
