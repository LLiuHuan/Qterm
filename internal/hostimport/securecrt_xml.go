package hostimport

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func DefaultSecureCRTXMLPath() string {
	return ""
}

func ParseSecureCRTXML(path string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultSecureCRTXMLPath()
	}
	if path == "" {
		return Result{}, ErrConfigNotFound
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, ErrConfigNotFound
		}
		return Result{}, fmt.Errorf("read securecrt xml: %w", err)
	}
	return ParseSecureCRTXMLContent(decodeLegacyText(raw))
}

type secureCRTFrame struct {
	name   string
	fields map[string]string
}

func ParseSecureCRTXMLContent(content string) (Result, error) {
	decoder := xml.NewDecoder(strings.NewReader(content))

	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	result := Result{Entries: make([]Entry, 0, 16)}
	stack := make([]secureCRTFrame, 0, 8)
	var (
		currentField string
		currentValue strings.Builder
		inValue      bool
	)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("parse securecrt xml: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "key":
				stack = append(stack, secureCRTFrame{
					name:   secureCRTNameAttr(element),
					fields: make(map[string]string, 8),
				})
			case "string", "dword":
				currentField = secureCRTNameAttr(element)
				currentValue.Reset()
				inValue = currentField != ""
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "key":
				if len(stack) == 0 {
					continue
				}
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if entry, ok := secureCRTSessionFromFrame(frame, stack); ok {
					entry.Line = len(result.Entries) + 1
					result.Entries = append(result.Entries, entry)
				}
			case "string", "dword":
				if inValue && len(stack) > 0 {
					frame := &stack[len(stack)-1]
					frame.fields[currentField] = strings.TrimSpace(currentValue.String())
				}
				currentField = ""
				currentValue.Reset()
				inValue = false
			}
		case xml.CharData:
			if inValue {
				currentValue.Write(element)
			}
		}
	}
	return result, nil
}

func secureCRTNameAttr(element xml.StartElement) string {
	for _, attribute := range element.Attr {
		if attribute.Name.Local == "name" {
			return strings.TrimSpace(attribute.Value)
		}
	}
	return ""
}

func secureCRTSessionFromFrame(frame secureCRTFrame, ancestors []secureCRTFrame) (Entry, bool) {
	sessionsIndex := -1
	for index := range ancestors {
		if ancestors[index].name == "Sessions" {
			sessionsIndex = index
			break
		}
	}
	if sessionsIndex < 0 {
		return Entry{}, false
	}
	if !strings.EqualFold(frame.fields["Protocol Name"], "SSH2") {
		return Entry{}, false
	}
	host := strings.TrimSpace(frame.fields["Hostname"])
	if host == "" {
		return Entry{}, false
	}
	port := 22
	portText := frame.fields["[SSH2] Port"]
	if portText == "" {
		portText = frame.fields["Port"]
	}
	if parsed, err := strconv.Atoi(strings.TrimSpace(portText)); err == nil && parsed > 0 && parsed <= 65535 {
		port = parsed
	}
	username := strings.TrimSpace(frame.fields["Username"])
	if username == "" {
		username = "root"
	}
	segments := make([]string, 0, len(ancestors)-sessionsIndex-1)
	for _, ancestor := range ancestors[sessionsIndex+1:] {
		segments = append(segments, ancestor.name)
	}
	name := strings.TrimSpace(frame.name)
	if name == "" {
		name = host
	}
	return Entry{
		Alias:    name,
		HostName: host,
		User:     username,
		Port:     port,
		Group:    joinGroupSegments(segments),
	}, true
}
