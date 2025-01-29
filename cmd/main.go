package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"log"

	"cloud.google.com/go/speech/apiv1/speechpb"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"cloud.google.com/go/vertexai/genai"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/audio"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/llm"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/tts"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/util"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
)

func main() {
	// Initialize
	// audioStream, err := audio.NewAudioStream()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer audioStream.Close()

	// tts, err := tts.NewTextToSpeech(getTtsOnRes(audioStream))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer tts.Close()

	llm, err := llm.NewLLMObject(vars.SystemPrompt)
	if err != nil {
		log.Fatal(err)
	}
	defer llm.Close()
	// llm.SetOnResponse(getLlmOnRes(tts))

	// stt, err := stt.NewSpeechToText(getSttOnRes(llm))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer stt.Close()
	// audioStream.SetOnInput(stt.OnInput)

	// Start
	// err = tts.Start()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer tts.Stop()

	// err = stt.Start()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer stt.Stop()

	// err = audioStream.Start()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer audioStream.Stop()

	llm.SetOnResponse(func(resp *genai.GenerateContentResponse) error {
		fmt.Println("onRes:")
		for _, part := range resp.Candidates[0].Content.Parts {
			fmt.Printf("Text: %s\n", part)
		}
		return nil
	})
	llm.GenerateContentStream("Hello, how are you?")

	for {
		time.Sleep(1 * time.Second)
	}
}

func getSttOnRes(llm *llm.LLMObject) func(resp *speechpb.StreamingRecognizeResponse) error {
	return func(resp *speechpb.StreamingRecognizeResponse) error {
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
				err := llm.GenerateContentStream(result.Alternatives[0].Transcript)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func getLlmOnRes(tts *tts.TextToSpeech) func(resp *genai.GenerateContentResponse) error {
	return func(resp *genai.GenerateContentResponse) error {
		for _, part := range resp.Candidates[0].Content.Parts {
			fmt.Printf("Text: %s\n", part)
			text := fmt.Sprint(part)

			err := tts.Send(text)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func getTtsOnRes(audioStream *audio.AudioStream) func(resp *texttospeechpb.StreamingSynthesizeResponse) error {
	return func(resp *texttospeechpb.StreamingSynthesizeResponse) error {
		output, err := util.BytesToInt16Binary(resp.AudioContent, binary.LittleEndian)
		if err != nil {
			return err
		}

		audioStream.Output(output)

		return nil
	}
}
