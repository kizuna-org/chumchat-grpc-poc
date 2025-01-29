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
	"github.com/kizuna-org/chumchat-grpc-poc/internal/stt"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/tts"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/util"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
)

func main() {
	// Initialize
	audioStream, err := audio.NewAudioStream()
	if err != nil {
		log.Fatal(err)
	}
	defer audioStream.Close()

	tts, err := tts.NewTextToSpeech()
	if err != nil {
		log.Fatal(err)
	}
	defer tts.Close()

	llm, err := llm.NewLLMObject(vars.SystemPrompt)
	if err != nil {
		log.Fatal(err)
	}
	defer llm.Close()
	llm.SetOnResponse(getLlmOnRes(tts, audioStream))

	stt, err := stt.NewSpeechToText(getSttOnRes(llm))
	if err != nil {
		log.Fatal(err)
	}
	defer stt.Close()
	audioStream.SetOnInput(stt.OnInput)

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

	for {
		time.Sleep(1 * time.Second)
	}
}

func getSttOnRes(llm *llm.LLMObject) func(*speechpb.StreamingRecognizeResponse) error {
	// lastText := ""

	return func(resp *speechpb.StreamingRecognizeResponse) error {
		fmt.Println("onRes:")
		for i, result := range resp.Results {
			if i != 0 {
				fmt.Println("multiple results")
				continue
			}
			if result.IsFinal {
				fmt.Println("Final")
			}

			fmt.Printf("Result: %+v\n", result)

			// if len(result.Alternatives) > 0 {
			// 	text := result.Alternatives[0].Transcript
			// 	dist := levenshtein.Distance(lastText, text)

			// 	if dist > vars.SSTSameDistance {
			// 		// err := llm.GenerateContentStream(text)
			// 		// if err != nil {
			// 		// 	return err
			// 		// }
			// 	}

			// 	lastText = text
			// }
		}
		return nil
	}
}

func getLlmOnRes(tts *tts.TextToSpeech, audioStream *audio.AudioStream) func(resp *genai.GenerateContentResponse) error {
	return func(resp *genai.GenerateContentResponse) error {
		for _, part := range resp.Candidates[0].Content.Parts {
			fmt.Printf("Text: %s\n", part)

			text := fmt.Sprint(part)
			data, err := tts.Speech(text)
			if err != nil {
				log.Fatal(err)
			}

			out, err := util.BytesToInt16Binary(data, binary.LittleEndian)
			if err != nil {
				log.Fatal(err)
			}

			audioStream.Output(out)
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
