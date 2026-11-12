package ai

import (
	"encoding/base64"
	"fmt"
	"strings"

	"changeme/internal/errno"
)

const (
	maxImageCount = 4

	maxImageBytes = 4 << 20
)

var allowedImageMediaTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/webp": {},
	"image/gif":  {},
}

type ImageInput struct {
	MediaType string

	DataBase64 string
}

func (image ImageInput) dataURL() string {
	return "data:" + image.MediaType + ";base64," + image.DataBase64
}

func imageInvalid(detail string) error {
	return errno.New(errno.CodeInvalidArg, msgAIImageInvalid, detail)
}

const msgAIImageInvalid = "ai.image.invalid"

func ParseImageDataURL(value string) (ImageInput, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ImageInput{}, imageInvalid("empty image")
	}
	if !strings.HasPrefix(trimmed, "data:") {

		return ImageInput{}, imageInvalid("image must be a data URL")
	}
	header, payload, found := strings.Cut(trimmed[len("data:"):], ",")
	if !found {
		return ImageInput{}, imageInvalid("data URL has no payload")
	}
	mediaType, encoded, found := strings.Cut(header, ";")
	if !found || strings.TrimSpace(encoded) != "base64" {
		return ImageInput{}, imageInvalid("data URL must be base64 encoded")
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if _, ok := allowedImageMediaTypes[mediaType]; !ok {
		return ImageInput{}, imageInvalid("unsupported image type " + mediaType)
	}

	if base64.StdEncoding.DecodedLen(len(payload)) > maxImageBytes+3 {
		return ImageInput{}, imageInvalid("image is too large")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {

		decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(payload, "="))
		if err != nil {
			return ImageInput{}, imageInvalid("image payload is not valid base64")
		}
	}
	if len(decoded) == 0 {
		return ImageInput{}, imageInvalid("image payload is empty")
	}
	if len(decoded) > maxImageBytes {
		return ImageInput{}, imageInvalid("image is too large")
	}
	return ImageInput{MediaType: mediaType, DataBase64: base64.StdEncoding.EncodeToString(decoded)}, nil
}

func countMessageImages(messages []Message) (int, error) {
	total := 0
	for _, message := range messages {
		total += len(message.Images)
	}
	if total > maxImageCount {
		return total, imageInvalid(fmt.Sprintf("at most %d images per request", maxImageCount))
	}
	return total, nil
}

func messageImages(message Message) ([]ImageInput, error) {
	if len(message.Images) == 0 {
		return nil, nil
	}
	images := make([]ImageInput, 0, len(message.Images))
	for index, raw := range message.Images {
		image, err := ParseImageDataURL(raw)
		if err != nil {
			return nil, errno.New(errno.CodeInvalidArg, msgAIImageInvalid, fmt.Sprintf("image #%d is invalid", index+1))
		}
		images = append(images, image)
	}
	return images, nil
}

func openAIChatMessages(messages []Message) ([]map[string]any, error) {
	converted := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		images, err := messageImages(message)
		if err != nil {
			return nil, err
		}
		if len(images) == 0 {
			converted = append(converted, map[string]any{"role": message.Role, "content": message.Content})
			continue
		}
		blocks := make([]map[string]any, 0, len(images)+1)
		if strings.TrimSpace(message.Content) != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": message.Content})
		}
		for _, image := range images {
			blocks = append(blocks, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": image.dataURL()},
			})
		}
		converted = append(converted, map[string]any{"role": message.Role, "content": blocks})
	}
	return converted, nil
}

func splitResponsesInput(messages []Message) (string, []map[string]any, error) {
	instructions := ""
	input := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		images, err := messageImages(message)
		if err != nil {
			return "", nil, err
		}
		if message.Role == "system" {
			if len(images) > 0 {
				return "", nil, imageInvalid("system messages cannot carry images")
			}
			instructions = strings.TrimSpace(instructions + "\n" + message.Content)
			continue
		}
		if len(images) == 0 {
			input = append(input, map[string]any{"role": message.Role, "content": message.Content})
			continue
		}
		blocks := make([]map[string]any, 0, len(images)+1)
		if strings.TrimSpace(message.Content) != "" {
			blocks = append(blocks, map[string]any{"type": "input_text", "text": message.Content})
		}
		for _, image := range images {
			blocks = append(blocks, map[string]any{
				"type":      "input_image",
				"image_url": image.dataURL(),
			})
		}
		input = append(input, map[string]any{"role": message.Role, "content": blocks})
	}
	return strings.TrimSpace(instructions), input, nil
}

func splitAnthropicMessages(messages []Message) (string, []map[string]any, error) {
	system := ""
	converted := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		images, err := messageImages(message)
		if err != nil {
			return "", nil, err
		}
		if message.Role == "system" {
			if len(images) > 0 {
				return "", nil, imageInvalid("system messages cannot carry images")
			}
			system = strings.TrimSpace(system + "\n" + message.Content)
			continue
		}
		if len(images) == 0 {
			converted = append(converted, map[string]any{"role": message.Role, "content": message.Content})
			continue
		}
		blocks := make([]map[string]any, 0, len(images)+1)
		if strings.TrimSpace(message.Content) != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": message.Content})
		}
		for _, image := range images {
			blocks = append(blocks, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": image.MediaType,
					"data":       image.DataBase64,
				},
			})
		}
		converted = append(converted, map[string]any{"role": message.Role, "content": blocks})
	}
	return strings.TrimSpace(system), converted, nil
}
