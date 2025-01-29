package main

import (
	"context"
	"fmt"
	"time"

	"log"

	"cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/audio"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/stt"
)

var ctx = context.Background()
var ttsClient *texttospeech.Client
var audioStream *audio.AudioStream

func main() {
	// Initialize
	stt, err := stt.NewSpeechToText(sttOnRes)
	if err != nil {
		log.Fatal(err)
	}
	defer stt.Close()

	audioStream, err := audio.NewAudioStream(stt.OnInput)
	if err != nil {
		log.Fatal(err)
	}
	defer audioStream.Close()

	// Start
	err = stt.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer stt.Stop()

	err = audioStream.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer audioStream.Stop()

	// ttsClient, err = texttospeech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer ttsClient.Close()
	for {
		time.Sleep(1 * time.Second)
	}
}

func sttOnRes(resp *speechpb.StreamingRecognizeResponse) error {
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

		// if len(result.Alternatives) > 0 {
		// 	trans := result.Alternatives[0].Transcript
		// 	go func() {
		// 		err := genLLMStreamContent(trans, func(resp *genai.GenerateContentResponse) error {

		// 			ttsStream, err := ttsClient.StreamingSynthesize(ctx)
		// 			if err != nil {
		// 				log.Fatal(err)
		// 			}
		// 			defer ttsStream.CloseSend()

		// 			ttsStream.Send(&texttospeechpb.StreamingSynthesizeRequest{
		// 				StreamingRequest: &texttospeechpb.StreamingSynthesizeRequest_StreamingConfig{
		// 					StreamingConfig: &texttospeechpb.StreamingSynthesizeConfig{
		// 						Voice: &texttospeechpb.VoiceSelectionParams{
		// 							LanguageCode: "en-US",
		// 							SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
		// 							Name:         "en-US-Journey-D",
		// 						},
		// 						StreamingAudioConfig: &texttospeechpb.StreamingAudioConfig{
		// 							AudioEncoding:   texttospeechpb.AudioEncoding_PCM,
		// 							SampleRateHertz: 16000,
		// 						},
		// 					},
		// 				},
		// 			})

		// 			go func() {
		// 				for {
		// 					resp, err := ttsStream.Recv()
		// 					if err == io.EOF {
		// 						continue
		// 					}
		// 					if err != nil {
		// 						return
		// 					}
		// 					if ttsStream.Context().Err() != nil {
		// 						return
		// 					}

		// 					output, err := BytesToInt16Binary(resp.AudioContent, binary.LittleEndian)
		// 					if err != nil {
		// 						log.Fatal(err)
		// 					}

		// 					audioStream.Output(output)
		// 				}
		// 			}()

		// 			for _, part := range resp.Candidates[0].Content.Parts {
		// 				fmt.Printf("Text: %s\n", part)
		// 				text := fmt.Sprint(part)

		// 				err = ttsStream.Send(&texttospeechpb.StreamingSynthesizeRequest{StreamingRequest: &texttospeechpb.StreamingSynthesizeRequest_Input{
		// 					Input: &texttospeechpb.StreamingSynthesisInput{
		// 						InputSource: &texttospeechpb.StreamingSynthesisInput_Text{
		// 							Text: text,
		// 						},
		// 					},
		// 				}})
		// 				if err != nil {
		// 					return err
		// 				}
		// 			}
		// 			return nil
		// 		})
		// 		if err != nil {
		// 			log.Fatal(err)
		// 		}
		// 	}()
		// }
	}

	return nil
}
