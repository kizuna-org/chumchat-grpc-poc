package main

import (
	"context"
	"fmt"

	"log"

	"cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"google.golang.org/api/option"
)

var ctx = context.Background()
var ttsClient *texttospeech.Client
var audioStream *AudioStream

func main() {
	audioStream, err := NewAudioStream()
	if err != nil {
		log.Fatal(err)
	}
	defer audioStream.Close()

	err = audioStream.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer audioStream.Stop()

	ttsClient, err = texttospeech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	if err != nil {
		log.Fatal(err)
	}
	defer ttsClient.Close()

	speechToTextFromMic(audioStream, sttOnRes)
}

func sttOnRes(resp *speechpb.StreamingRecognizeResponse) {
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
}
