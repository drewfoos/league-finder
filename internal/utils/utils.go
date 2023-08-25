package utils

import (
	"log"
	"os"
	"strings"
)

func GetApiKeyFromFile(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "api_key=") {
			return strings.TrimPrefix(line, "api_key=")
		}
	}
	return ""
}

func safeString(data map[string]interface{}, key string) string {
	if value, ok := data[key]; ok {
		return value.(string)
	}
	return ""
}

func safeInt(data map[string]interface{}, key string) int {
	if value, ok := data[key]; ok && value != nil {
		return int(value.(float64))
	}
	return 0
}
