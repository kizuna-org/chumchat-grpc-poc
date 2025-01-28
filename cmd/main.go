package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"log"

	"cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"github.com/gordonklaus/portaudio"
	"google.golang.org/api/option"

	"cloud.google.com/go/vertexai/genai"
)

const sysPrompt = `
Please interact in English.
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
	FramesPerBuffer = 1024
)

const (
	VadFinishWaitSTT = 50 // ms
)

type AudioStream struct {
	stream *portaudio.Stream
	input  []int16
	output []int16
}

func main() {
	portaudio.Initialize()
	defer portaudio.Terminate()

	input := make([]int16, FramesPerBuffer)
	output := make([]int16, FramesPerBuffer)

	var globalOutputMutex sync.Mutex
	var globalOutput []int16

	stream, err := portaudio.OpenDefaultStream(1, 1, SampleRate, FramesPerBuffer, &input, &output)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	audioStream := &AudioStream{
		stream: stream,
		input:  input,
		output: output,
	}

	err = stream.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Stop()

	ctx := context.Background()

	ttsClient, err := texttospeech.NewClient(ctx, option.WithCredentialsFile("./chumchat.json"))
	if err != nil {
		log.Fatal(err)
	}
	defer ttsClient.Close()

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
					err := generateContentFromText(trans, func(resp *genai.GenerateContentResponse) error {

						ttsStream, err := ttsClient.StreamingSynthesize(ctx)
						if err != nil {
							log.Fatal(err)
						}
						defer ttsStream.CloseSend()

						ttsStream.Send(&texttospeechpb.StreamingSynthesizeRequest{
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

						go func() {
							for {
								resp, err := ttsStream.Recv()
								if err == io.EOF {
									continue
								}
								if err != nil {
									return
								}
								if ttsStream.Context().Err() != nil {
									return
								}

								output, err = BytesToInt16Binary(resp.AudioContent, binary.LittleEndian)
								if err != nil {
									log.Fatal(err)
								}

								globalOutputMutex.Lock()
								globalOutput = output
								globalOutputMutex.Unlock()
							}
						}()

						for _, part := range resp.Candidates[0].Content.Parts {
							fmt.Printf("Text: %s\n", part)
							text := fmt.Sprint(part)

							err = ttsStream.Send(&texttospeechpb.StreamingSynthesizeRequest{StreamingRequest: &texttospeechpb.StreamingSynthesizeRequest_Input{
								Input: &texttospeechpb.StreamingSynthesisInput{
									InputSource: &texttospeechpb.StreamingSynthesisInput_Text{
										Text: text,
									},
								},
							}})
							if err != nil {
								return err
							}
						}
						return nil
					})
					if err != nil {
						log.Fatal(err)
					}
				}()
			}
		}
	}

	go func() {
		for {
			globalOutputMutex.Lock()
			if len(globalOutput) == 0 {
				output = make([]int16, FramesPerBuffer) // 出力バッファを0で初期化
				copy(audioStream.output, output)        // audioStream.outputにコピー
				err := audioStream.stream.Write()       // 出力バッファを再生
				if err != nil {
					log.Printf("stream.Write() failed: %v", err) // エラーログをPrintfに変更 (Fatalではない)
					// エラーが発生しても処理を継続 (必要に応じてエラー処理を追加)
				}

				globalOutputMutex.Unlock()
				time.Sleep(time.Millisecond * 10) // 必要に応じてsleepを挟むことでCPU使用率を下げられます
				continue
			}

			copyLength := FramesPerBuffer
			if len(globalOutput) < FramesPerBuffer {
				copyLength = len(globalOutput)
			}

			copiedData := make([]int16, FramesPerBuffer) // 固定長バッファを作成
			copy(copiedData, globalOutput[:copyLength])  // globalOutputからコピー

			// globalOutputの長さがFramesPerBufferより短い場合、残りを0で埋める (既に0で初期化されているので不要)
			// 必要であれば明示的に0埋めしても良いですが、makeで初期化された時点で0なので通常は不要です

			copy(audioStream.output, copiedData) // audioStream.outputにコピー

			globalOutput = globalOutput[copyLength:] // globalOutputからコピーした要素を削除
			globalOutputMutex.Unlock()

			fmt.Printf("Copied %d elements to audioStream.output. Remaining globalOutput: %d\n", copyLength, len(globalOutput))
			// ここで audioStream.output を使用する処理を記述 (例: 再生処理など)

			err := audioStream.stream.Write()
			if err != nil {
				log.Printf("stream.Write() failed: %v", err) // エラーログをPrintfに変更 (Fatalではない)
				// エラーが発生しても処理を継続 (必要に応じてエラー処理を追加)
			}
		}
	}()

	speechToTextFromMic(audioStream, onRes)
}
