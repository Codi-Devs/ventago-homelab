package workers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type hwSnapshot struct {
	At        time.Time
	PackageC  float64
	FanRPM    int
	PWM       int
	PWMPct    int
	OKPackage bool
	OKFan     bool
}

func readHwSnapshot() hwSnapshot {
	s := hwSnapshot{At: time.Now().UTC()}
	names, err := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	if err != nil {
		return s
	}
	for _, nameFile := range names {
		raw, readErr := os.ReadFile(nameFile)
		if readErr != nil {
			continue
		}
		dir := filepath.Dir(nameFile)
		switch strings.TrimSpace(string(raw)) {
		case "coretemp":
			if c, ok := readCoretempPackageC(dir); ok {
				s.PackageC = c
				s.OKPackage = true
			}
		case "dell_smm":
			if rpm, ok := readSysfsInt(filepath.Join(dir, "fan1_input")); ok {
				s.FanRPM = rpm
				s.OKFan = true
			}
			if pwm, ok := readSysfsInt(filepath.Join(dir, "pwm1")); ok {
				s.PWM = pwm
				s.PWMPct = pwm * 100 / 255
			}
		}
	}
	return s
}

func readCoretempPackageC(dir string) (float64, bool) {
	labels, err := filepath.Glob(filepath.Join(dir, "temp*_label"))
	if err != nil {
		return 0, false
	}
	for _, labelFile := range labels {
		b, err := os.ReadFile(labelFile)
		if err != nil {
			continue
		}
		if !strings.Contains(strings.ToLower(string(b)), "package") {
			continue
		}
		base := strings.TrimSuffix(labelFile, "_label")
		milli, ok := readSysfsInt(base + "_input")
		if !ok {
			return 0, false
		}
		return float64(milli) / 1000, true
	}
	milli, ok := readSysfsInt(filepath.Join(dir, "temp1_input"))
	if !ok {
		return 0, false
	}
	return float64(milli) / 1000, true
}

func readSysfsInt(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, false
	}
	return n, true
}
