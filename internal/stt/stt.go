package stt

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/audio"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/util"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
	"github.com/kizuna-org/go-webrtcvad"
	"google.golang.org/api/option"
)

func speechToTextFromMic(audioStream *audio.AudioStream, onRes func(*speechpb.StreamingRecognizeResponse)) {
	var lastRes *speechpb.StreamingRecognizeResponse = nil
	var frameActive = false
	var lastActiveTime = time.Now()

	vadInst := webrtcvad.Create()
	defer webrtcvad.Free(vadInst)
	err := webrtcvad.Init(vadInst)
	if err != nil {
		log.Fatal(err)
	}
	err = webrtcvad.SetMode(vadInst, vars.VadMode)
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

	// TODO: これをaudio.NewAudioStream()でやる
	// というか、sttの分岐だけを切り出して別のstructにしたほうが良さそう
	// stt.New(中でgoogle sttの初期化)して、stt.onInputをaudioStream.onInputに渡す感じ
	go func(input []int16) error {
		// buf := make([]byte, SampleRate/1000*FrameDuration*BitDepth/8) //1024)

		if err != nil && err != io.EOF {
			return err
		}

		frame, err := util.Int16ToBytesBinary(input, binary.LittleEndian)
		if err != nil {
			return err
		}

		frameActive, err = webrtcvad.Process(vadInst, vars.SampleRate, frame, 16000/1000*20)
		if err != nil {
			log.Fatal(err)
		}

		if frameActive {
			lastActiveTime = time.Now()
		}

		if !frameActive && time.Since(lastActiveTime) > vars.VadFinishWaitSTT*time.Millisecond {
			if lastRes != nil && onRes != nil {
				fmt.Println("\n\n\n\n\n\n\n\n\n\n\n\n\n\nLatency: ", time.Since(lastActiveTime))
				onRes(lastRes)
				lastRes = nil
			}

			return nil
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
			if err := stream.CloseSend(); err != nil {
				log.Fatalf("Could not close stream: %v", err)
			}
			return nil
		}
		if err != nil {
			log.Printf("Could not read from stdin: %v", err)
			return nil
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
