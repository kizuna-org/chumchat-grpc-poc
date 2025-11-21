package llm

import (
	"context"

	"cloud.google.com/go/vertexai/genai"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
	"google.golang.org/api/iterator"
)

type LLMObject struct {
	client *genai.Client
	gemini *genai.GenerativeModel

	ctx context.Context

	onResponse func(*genai.GenerateContentResponse) error
}

func NewLLMObject(prompt string) (*LLMObject, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, vars.ProjectID, vars.Location)
	if err != nil {
		return nil, err
	}
	gemini := client.GenerativeModel(vars.GenAIModelName)
	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(prompt)},
	}

	return &LLMObject{
		client:     client,
		gemini:     gemini,
		ctx:        ctx,
		onResponse: nil,
	}, nil
}

func (llm *LLMObject) Close() error {
	return llm.client.Close()
}

func (llm *LLMObject) GenerateContentStream(text string) error {
	prompt := genai.Text(text)

	iter := llm.gemini.GenerateContentStream(llm.ctx, prompt)
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		if llm.onResponse != nil {
			err = llm.onResponse(resp)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (llm *LLMObject) SetOnResponse(onResponse func(*genai.GenerateContentResponse) error) {
	llm.onResponse = onResponse
}
