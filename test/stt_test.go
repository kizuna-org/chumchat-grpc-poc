package main

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"google.golang.org/api/option"
)

func TestMain(m *testing.M) {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	code := m.Run()
	os.Exit(code)
}

func BenchmarkGoogleSTT(b *testing.B) {
	ctx := context.Background()
	client, err := speech.NewClient(ctx, option.WithCredentialsFile("../chumchat.json"))
	if err != nil {
		b.Fatal(err)
	}

	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		b.Fatal(err)
	}
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: 16000,
					LanguageCode:    "ja-JP",
				},
			},
		},
	}); err != nil {
		b.Fatal(err)
	}

	f, err := os.Open("../hello2.wav")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	b.ResetTimer()

	var count = 0

	for i := 0; i < b.N; i++ {
		go func() error {
			buf := make([]byte, 1024)
			for {
				n, err := f.Read(buf)
				if n > 0 {
					if err := stream.Send(&speechpb.StreamingRecognizeRequest{
						StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
							AudioContent: buf[:n],
						},
					}); err != nil {
						return err
					}
				}
				if err == io.EOF {
					if err := stream.CloseSend(); err != nil {
						return err
					}
					return nil
				}
				if err != nil {
					continue
				}
			}
		}()
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			b.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			b.Fatalf("Could not recognize: %v", err)
		}

		count++

		if count == b.N {
			break
		}
	}
}

func BenchmarkWhisper(b *testing.B) {
	client := openai.NewClient()
	ctx := context.Background()
	file, err := os.Open("../hello2.wav")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.Audio.Transcriptions.New(ctx, openai.AudioTranscriptionNewParams{
			Model: openai.F(openai.AudioModelWhisper1),
			File:  openai.F[io.Reader](file),
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
