package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"log"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"github.com/gordonklaus/portaudio"
	"github.com/kizuna-org/go-webrtcvad"
	"google.golang.org/api/iterator"
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

func speechToTextFromMic(audioStream *AudioStream, onRes func(*speechpb.StreamingRecognizeResponse)) {
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
	defer client.Close()

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
		// buf := make([]byte, SampleRate/1000*FrameDuration*BitDepth/8) //1024)

		for {
			err := audioStream.stream.Read()
			if err != nil && err != io.EOF {
				log.Printf("Could not read from stdin: %v", err)
				break
			}

			frame, err := Int16ToBytesBinary(audioStream.input, binary.LittleEndian)
			if err != nil {
				log.Fatal(err)
				break
			}

			frameActive, err = webrtcvad.Process(vadInst, SampleRate, frame, 16000/1000*20)
			if err != nil {
				log.Fatal(err)
			}

			// fmt.Println("Frame Active: ", frameActive)

			if frameActive {
				lastActiveTime = time.Now()
			}

			if !frameActive && time.Since(lastActiveTime) > VadFinishWaitSTT*time.Millisecond {
				// fmt.Fprintln(os.Stderr, time.Now(), "Finish")
				if lastRes != nil && onRes != nil {
					fmt.Println("\n\n\n\n\n\n\n\n\n\n\n\n\n\nLatency: ", time.Since(lastActiveTime))
					onRes(lastRes)
					lastRes = nil
				}

				continue
			} else {
				// fmt.Fprintln(os.Stderr, time.Now(), "active")
			}

			if err := stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
					AudioContent: frame,
				},
			}); err != nil {
				log.Printf("Could not send audio: %v", err)
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

		// fmt.Fprintln(os.Stderr, time.Now(), "Response: ", resp)
	}
}

func generateContentFromText(text string, onRes func(*genai.GenerateContentResponse) error) error {
	fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	modelName := "gemini-1.5-flash"
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

		err = onRes(resp)
		if err != nil {
			return err
		}
	}

	return nil
}

func Int16ToBytesBinary(ints []int16, order binary.ByteOrder) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, order, ints); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func BytesToInt16Binary(data []byte, order binary.ByteOrder) ([]int16, error) {
	reader := bytes.NewReader(data)
	int16Slice := make([]int16, len(data)/2)
	if err := binary.Read(reader, order, int16Slice); err != nil {
		return nil, err
	}
	return int16Slice, nil
}
