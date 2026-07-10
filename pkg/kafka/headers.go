package kafka

import (
	"errors"
	"strconv"
	"strings"
	"time"

	kgo "github.com/segmentio/kafka-go"
)

func HeaderValue(headers []kgo.Header, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}

func AppendHeaders(headers []kgo.Header, extras ...kgo.Header) []kgo.Header {
	result := make([]kgo.Header, 0, len(headers)+len(extras))
	result = append(result, headers...)
	result = append(result, extras...)
	return result
}

func ParseIntHeader(headers []kgo.Header, key string) (int, error) {
	value := strings.TrimSpace(HeaderValue(headers, key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New("invalid kafka header " + key + ": " + value)
	}
	return parsed, nil
}

func ParseInt64Header(headers []kgo.Header, key string) (int64, error) {
	value := strings.TrimSpace(HeaderValue(headers, key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errors.New("invalid kafka header " + key + ": " + value)
	}
	return parsed, nil
}

func ParseRFC3339Header(headers []kgo.Header, key string) (time.Time, error) {
	value := strings.TrimSpace(HeaderValue(headers, key))
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New("invalid kafka header " + key + ": " + value)
	}
	return parsed, nil
}
