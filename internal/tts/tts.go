package tts

import (
	"context"
	"log"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
	"google.golang.org/api/option"
)

type TextToSpeech struct {
	client *texttospeech.Client

	ctx context.Context
}

func NewTextToSpeech() (*TextToSpeech, error) {
	ctx := context.Background()

	client, err := texttospeech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	if err != nil {
		log.Fatal(err)
	}

	return &TextToSpeech{
		client: client,
		ctx:    ctx,
	}, nil
}

func (tts *TextToSpeech) Close() error {
	tts.ctx.Done()

	return tts.client.Close()
}

func (tts *TextToSpeech) Speech(text string) ([]byte, error) {
	req := texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-US",
			Name:         "en-US-Journey-F",
			// SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding:   texttospeechpb.AudioEncoding_LINEAR16,
			SampleRateHertz: vars.SampleRate,
			SpeakingRate:    1.0,
		},
	}

	resp, err := tts.client.SynthesizeSpeech(tts.ctx, &req)
	if err != nil {
		log.Fatal(err)
	}

	return resp.AudioContent, nil
}
