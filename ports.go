package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func parsePorts(ports string) ([]int, error) {
	var portList []int
	parts := strings.Split(ports, ",")
	for _, part := range parts {
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range: %s", part)
			}

			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid start port: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid end port: %s", rangeParts[1])
			}

			if start > end {
				return nil, fmt.Errorf("start port must be less than or equal to end port: %s", part)
			}

			for port := start; port <= end; port++ {
				portList = append(portList, port)
			}
		} else {
			port, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", part)
			}
			portList = append(portList, port)
		}
	}
	return portList, nil
}

func expandAtRefs(spec string) (string, error) {
	if !strings.Contains(spec, "@") {
		return spec, nil
	}
	parts := strings.Split(spec, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "@") {
			fileSpec, err := loadPortsFromFile(part[1:])
			if err != nil {
				return "", fmt.Errorf("expand %s: %w", part, err)
			}
			result = append(result, fileSpec)
		} else {
			result = append(result, part)
		}
	}
	return strings.Join(result, ","), nil
}

func loadPortsFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read port file: %w", err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, ","), nil
}
