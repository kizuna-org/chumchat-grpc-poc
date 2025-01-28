package llm

import (
	"context"

	"cloud.google.com/go/vertexai/genai"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
	"google.golang.org/api/iterator"
)

func genLLMStreamContent(text string, onRes func(*genai.GenerateContentResponse) error) error {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, vars.ProjectID, vars.Location)
	if err != nil {
		return err
	}
	gemini := client.GenerativeModel(vars.GenAIModelName)
	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(vars.SystemPrompt)},
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
