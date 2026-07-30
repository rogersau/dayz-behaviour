package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

func loadServerAuthMap(path string) (map[string]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read server auth file: %w", err)
	}
	var authMap map[string]string
	if err := json.Unmarshal(data, &authMap); err != nil {
		return nil, fmt.Errorf("decode server auth file: %w", err)
	}
	if len(authMap) == 0 {
		return nil, errors.New("server auth file must contain at least one server")
	}
	for serverID, value := range authMap {
		if strings.TrimSpace(serverID) == "" || strings.TrimSpace(value) == "" {
			return nil, errors.New("server auth file contains an empty server ID or value")
		}
	}
	return authMap, nil
}
