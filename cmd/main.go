package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"log"
	"os"
	"path/filepath"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"github.com/kizuna-org/go-webrtcvad"
	"google.golang.org/api/option"

	"cloud.google.com/go/vertexai/genai"
)

func main() {
	start := time.Now()
	onRes := func(resp *speechpb.StreamingRecognizeResponse) {
		fmt.Println("time:", time.Now().UnixMilli()-start.UnixMilli())
		for i, result := range resp.Results {
			if i != 0 {
				fmt.Println("multiple results")
				continue
			}
			if result.IsFinal {
				continue
			}

			fmt.Printf("Result: %+v\n", result)

			// if len(result.Alternatives) > 0 {
			// 	trans := result.Alternatives[0].Transcript
			// 	ret, err := generateContentFromText(trans, "chumchat")
			// 	if err != nil {
			// 		log.Fatal(err)
			// 	}

			// 	for _, part := range ret.Candidates[0].Content.Parts {
			// 		fmt.Printf("Text: %s\n", part)
			// 		fmt.Println("time:", time.Now().UnixMilli()-start.UnixMilli())
			// 		filename, err := generateSpeech(fmt.Sprint(part))
			// 		if err != nil {
			// 			log.Fatal(err)
			// 		}
			// 		fmt.Printf("Audio file: %s\n", filename)
			// 		fmt.Println("time:", time.Now().UnixMilli()-start.UnixMilli())
			// 	}
			// }
		}
	}

	if len(os.Args) > 1 {
		speechToTextFromFile(onRes)
	} else {
		speechToTextFromMic(onRes)
	}
}

func speechToTextFromMic(onRes func(*speechpb.StreamingRecognizeResponse)) {
	const (
		// VadMode vad mode
		VadMode = 3
		// SampleRate sample rate
		SampleRate = 16000
		// BitDepth bit depth
		BitDepth = 16
		// FrameDuration frame duration
		FrameDuration = 20
	)

	var lastRes *speechpb.StreamingRecognizeResponse

	vadInst := webrtcvad.Create()
	defer webrtcvad.Free(vadInst)
	err := webrtcvad.Init(vadInst)
	if err != nil {
		log.Fatal(err)
	}
	err = webrtcvad.SetMode(vadInst, VadMode)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	client, err := speech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	if err != nil {
		log.Fatal(err)
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				InterimResults: true,
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: 16000,
					LanguageCode:    "ja-JP",
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	go func() {
		buf := make([]byte, SampleRate/1000*FrameDuration*BitDepth/8) //1024)

		for {
			n, err := os.Stdin.Read(buf)
			if err != nil && err != io.EOF {
				log.Printf("Could not read from stdin: %v", err)
				break
			}

			if n > 0 {
				frame := buf[:n]

				frameActive, err := webrtcvad.Process(vadInst, SampleRate, frame, 16000/1000*20)
				if err != nil {
					log.Fatal(err)
				}

				// fmt.Println("Frame Active: ", frameActive)

				if lastRes != nil && !frameActive {
					if onRes != nil {
						onRes(lastRes)
						lastRes = nil
					}
				}

				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
						AudioContent: frame,
					},
				}); err != nil {
					log.Printf("Could not send audio: %v", err)
				}
			}
			if err == io.EOF {
				// Nothing else to pipe, close the stream.
				if err := stream.CloseSend(); err != nil {
					log.Fatalf("Could not close stream: %v", err)
				}
				return
			}
			if err != nil {
				log.Printf("Could not read from stdin: %v", err)
				continue
			}
		}
	}()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			if err.Code == 3 || err.Code == 11 {
				log.Print("WARNING: Speech recognition request exceeded limit of 60 seconds.")
			}
			log.Fatalf("Could not recognize: %v", err)
		}

		lastRes = resp
	}
}

func speechToTextFromFile(onRes func(*speechpb.StreamingRecognizeResponse)) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <AUDIOFILE>\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "<AUDIOFILE> must be a path to a local audio file. Audio file must be a 16-bit signed little-endian encoded with a sample rate of 16000.\n")

	}
	flag.Parse()
	if len(flag.Args()) != 1 {
		log.Fatal("Please pass path to your local audio file as a command line argument")
	}
	audioFile := flag.Arg(0)

	ctx := context.Background()

	client, err := speech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	if err != nil {
		log.Fatal(err)
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	f, err := os.Open(audioFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
						AudioContent: buf[:n],
					},
				}); err != nil {
					log.Printf("Could not send audio: %v", err)
				}
			}
			if err == io.EOF {
				if err := stream.CloseSend(); err != nil {
					log.Fatalf("Could not close stream: %v", err)
				}
				return
			}
			if err != nil {
				log.Printf("Could not read from %s: %v", audioFile, err)
				continue
			}
		}
	}()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			log.Fatalf("Could not recognize: %v", err)
		}
		if onRes != nil {
			onRes(resp)
		}
	}
}

func generateContentFromText(text string, projectID string) (*genai.GenerateContentResponse, error) {
	modelName := "gemini-2.0-flash-exp"
	location := "us-central1"

	ctx := context.Background()
	client, err := genai.NewClient(ctx, projectID, location)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}
	gemini := client.GenerativeModel(modelName)
	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(`
		Please interact in Japanese.
		`)},
	}
	prompt := genai.Text(text)

	resp, err := gemini.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("error generating content: %w", err)
	}
	return resp, nil
}

func generateSpeech(text string) (string, error) {
	ctx := context.Background()

	client, err := texttospeech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	if err != nil {
		return "", err
	}
	defer client.Close()

	req := texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "ja-JP",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
			SpeakingRate:  1.5,
		},
	}

	resp, err := client.SynthesizeSpeech(ctx, &req)
	if err != nil {
		return "", err
	}

	filename := "output.mp3"
	err = os.WriteFile(filename, resp.AudioContent, 0644)
	if err != nil {
		return "", err
	}
	fmt.Printf("Audio content written to file: %v\n", filename)
	return filename, nil
}
