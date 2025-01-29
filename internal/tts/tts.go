package tts

import (
	"context"
	"log"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"google.golang.org/api/option"
)

type TextToSpeech struct {
	client *texttospeech.Client
	stream *texttospeechpb.TextToSpeech_StreamingSynthesizeClient

	ctx context.Context

	onResponse func(*texttospeechpb.StreamingSynthesizeResponse) error
}

func NewTextToSpeech(onResponse func(*texttospeechpb.StreamingSynthesizeResponse) error) (*TextToSpeech, error) {
	ctx := context.Background()

	client, err := texttospeech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	if err != nil {
		log.Fatal(err)
	}

	stream, err := client.StreamingSynthesize(ctx)
	if err != nil {
		log.Fatal(err)
	}

	stream.Send(&texttospeechpb.StreamingSynthesizeRequest{
		StreamingRequest: &texttospeechpb.StreamingSynthesizeRequest_StreamingConfig{
			StreamingConfig: &texttospeechpb.StreamingSynthesizeConfig{
				Voice: &texttospeechpb.VoiceSelectionParams{
					LanguageCode: "en-US",
					SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
					Name:         "en-US-Journey-D",
				},
				StreamingAudioConfig: &texttospeechpb.StreamingAudioConfig{
					AudioEncoding:   texttospeechpb.AudioEncoding_PCM,
					SampleRateHertz: 16000,
				},
			},
		},
	})

	return &TextToSpeech{
		client:     client,
		stream:     &stream,
		ctx:        context.Background(),
		onResponse: onResponse,
	}, nil
}

func (tts *TextToSpeech) Close() error {
	return tts.client.Close()
}

func (tts *TextToSpeech) Start() error {
	go func() {
		for {
			select {
			case <-tts.ctx.Done():
				return
			default:
				resp, err := (*tts.stream).Recv()
				if err != nil {
					log.Printf("streaming failed: %v", err)
				}

				// FIXME: Sendされたのから到着順が保証されない&複数のSendを同時に受信して音声が混ざる可能性がある
				err = tts.onResponse(resp)
				if err != nil {
					log.Printf("onResponse failed: %v", err)
				}
			}
		}
	}()

	return nil
}

func (tts *TextToSpeech) Stop() error {
	tts.ctx.Done()
	(*tts.stream).CloseSend()

	return nil
}

func (tts *TextToSpeech) Send(text string) error {
	return (*tts.stream).Send(&texttospeechpb.StreamingSynthesizeRequest{StreamingRequest: &texttospeechpb.StreamingSynthesizeRequest_Input{
		Input: &texttospeechpb.StreamingSynthesisInput{
			InputSource: &texttospeechpb.StreamingSynthesisInput_Text{
				Text: text,
			},
		},
	}})
}
