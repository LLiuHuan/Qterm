package tunnel

import (
	"io"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func normalizeTerminalEncoding(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "gbk") {
		return "gbk"
	}
	return "utf-8"
}

func NewTerminalInputEncoder(target io.Writer, encoding string) (io.Writer, error) {
	if normalizeTerminalEncoding(encoding) != "gbk" {
		return target, nil
	}
	return transform.NewWriter(target, simplifiedchinese.GBK.NewEncoder()), nil
}

func NewTerminalOutputReader(source io.Reader, encoding string) (io.Reader, error) {
	if normalizeTerminalEncoding(encoding) != "gbk" {
		return source, nil
	}

	return transform.NewReader(source, simplifiedchinese.GBK.NewDecoder()), nil
}

func encodeTerminalInput(value string, encoding string) ([]byte, error) {
	if normalizeTerminalEncoding(encoding) != "gbk" {
		return []byte(value), nil
	}
	encoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(value))
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func decodeTerminalOutput(value []byte, encoding string) ([]byte, error) {
	if normalizeTerminalEncoding(encoding) != "gbk" {
		return value, nil
	}
	decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), value)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
