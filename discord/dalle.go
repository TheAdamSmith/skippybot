package discord

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// TODO: find better name
func GetImgUrl(prompt string, client *openai.Client) (string, error) {
	imgReq := openai.ImageRequest{
		Prompt:         prompt,
		Size:           openai.CreateImageSize256x256,
		ResponseFormat: openai.CreateImageResponseFormatURL,
		N:              1,
	}

	resp, err := client.CreateImage(context.Background(), imgReq)
	if err != nil {
		return "", fmt.Errorf("unable to get image url: %s", err)
	}

	return resp.Data[0].URL, nil
}
