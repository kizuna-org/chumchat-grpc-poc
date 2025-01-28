package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"log"
	"os"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"github.com/kizuna-org/go-webrtcvad"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"cloud.google.com/go/vertexai/genai"
)

const sysPrompt = `
Please interact in Japanese.
Please respond in 1-2 sentences.
`

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

const (
	VadFinishWaitSTT = 50 // ms
)

func main() {
	onRes := func(resp *speechpb.StreamingRecognizeResponse) {
		fmt.Println("onRes:")
		for i, result := range resp.Results {
			if i != 0 {
				fmt.Println("multiple results")
				continue
			}
			if result.IsFinal {
				// fmt.Println("Final")
				continue
			}

			fmt.Printf("Result: %+v\n", result)

			if len(result.Alternatives) > 0 {
				trans := result.Alternatives[0].Transcript
				go func() {
					err := generateContentFromText(trans, func(resp *genai.GenerateContentResponse) {
						for _, part := range resp.Candidates[0].Content.Parts {
							fmt.Printf("Text: %s\n", part)
							// filename, err := generateSpeech(fmt.Sprint(part))
							// if err != nil {
							// 	log.Fatal(err)
							// }
							// fmt.Printf("Audio file: %s\n", filename)
						}
					})
					if err != nil {
						log.Fatal(err)
					}
				}()
			}
		}
	}

	speechToTextFromMic(onRes)
}

func speechToTextFromMic(onRes func(*speechpb.StreamingRecognizeResponse)) {
	var lastRes *speechpb.StreamingRecognizeResponse = nil
	var frameActive = false
	var lastActiveTime = time.Now()

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

				frameActive, err = webrtcvad.Process(vadInst, SampleRate, frame, 16000/1000*20)
				if err != nil {
					log.Fatal(err)
				}

				// fmt.Println("Frame Active: ", frameActive)

				if frameActive {
					lastActiveTime = time.Now()
				}

				if !frameActive && time.Since(lastActiveTime) > VadFinishWaitSTT*time.Millisecond {
					fmt.Fprintln(os.Stderr, time.Now(), "Finish")
					if lastRes != nil && onRes != nil {
						fmt.Println("\n\n\n\n\n\n\n\n\n\n\n\n\n\nLatency: ", time.Since(lastActiveTime))
						onRes(lastRes)
						lastRes = nil
					}

					continue
				} else {
					fmt.Fprintln(os.Stderr, time.Now(), "active")
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

		fmt.Fprintln(os.Stderr, time.Now(), "Response: ", resp)
	}
}

func generateContentFromText(text string, onRes func(*genai.GenerateContentResponse)) error {
	modelName := "gemini-2.0-flash-exp"
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
		onRes(resp)
	}

	return nil
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
