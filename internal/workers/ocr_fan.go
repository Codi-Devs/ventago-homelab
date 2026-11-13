package workers

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const ocrFanBoostPWM = 255

func boostOCRFan() func() {
	pwmPath := findDellPWMPath()
	if pwmPath == "" {
		return func() {}
	}
	raw, err := os.ReadFile(pwmPath)
	if err != nil {
		log.Printf("ocr fan: read %s: %v", pwmPath, err)
		return func() {}
	}
	prev, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return func() {}
	}
	if prev >= ocrFanBoostPWM {
		return func() {}
	}
	if err := os.WriteFile(pwmPath, []byte(strconv.Itoa(ocrFanBoostPWM)), 0o644); err != nil {
		log.Printf("ocr fan: boost %s: %v", pwmPath, err)
		return func() {}
	}
	log.Printf("ocr fan: pwm %d -> %d", prev, ocrFanBoostPWM)
	return func() {
		if err := os.WriteFile(pwmPath, []byte(strconv.Itoa(prev)), 0o644); err != nil {
			log.Printf("ocr fan: restore %s: %v", pwmPath, err)
		}
	}
}

func findDellPWMPath() string {
	if p := strings.TrimSpace(os.Getenv("OCR_FAN_PWM_PATH")); p != "" {
		return p
	}
	names, err := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	if err != nil {
		return ""
	}
	for _, nameFile := range names {
		b, readErr := os.ReadFile(nameFile)
		if readErr != nil {
			continue
		}
		if strings.TrimSpace(string(b)) != "dell_smm" {
			continue
		}
		pwm := filepath.Join(filepath.Dir(nameFile), "pwm1")
		if _, statErr := os.Stat(pwm); statErr == nil {
			return pwm
		}
	}
	return ""
}
