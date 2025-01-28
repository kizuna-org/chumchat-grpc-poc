package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/api/iterator"
)

func generateContentFromText(text string, onRes func(*genai.GenerateContentResponse) error) error {
	fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	modelName := "gemini-1.5-flash"
	location := "us-central1"
	projectID := "chumchat"

	ctx := context.Background()
	client, err := genai.NewClient(ctx, projectID, location)
	if err != nil {
		return err
	}
	gemini := client.GenerativeModel(modelName)
	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(sysPrompt)},
	}
	prompt := genai.Text(text)

	iter := gemini.GenerateContentStream(ctx, prompt)
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		err = onRes(resp)
		if err != nil {
			return err
		}
	}

	return nil
}
