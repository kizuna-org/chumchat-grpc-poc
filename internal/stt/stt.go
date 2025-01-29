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
	"github.com/kizuna-org/chumchat-grpc-poc/internal/util"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
	"github.com/kizuna-org/go-webrtcvad"
	"google.golang.org/api/option"
)

type SpeechToText struct {
	vadInst *webrtcvad.VadInst
	stream  *speechpb.Speech_StreamingRecognizeClient
	client  *speech.Client

	ctx context.Context

	lastRes      *speechpb.StreamingRecognizeResponse
	inactiveTime time.Time

	onResponse func(*speechpb.StreamingRecognizeResponse) error
}

func NewSpeechToText(onResponse func(*speechpb.StreamingRecognizeResponse) error) (*SpeechToText, error) {
	vadInst := webrtcvad.Create()
	err := webrtcvad.Init(vadInst)
	if err != nil {
		return nil, err
	}
	err = webrtcvad.SetMode(vadInst, vars.VadMode)
	if err != nil {
		return nil, err
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
					LanguageCode:    "en-US", //"ja-JP",
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	return &SpeechToText{
		vadInst:      &vadInst,
		client:       client,
		stream:       &stream,
		ctx:          context.Background(),
		lastRes:      nil,
		inactiveTime: time.Now(),
		onResponse:   onResponse,
	}, nil
}

func (st *SpeechToText) Close() error {
	if err := (*st.stream).CloseSend(); err != nil {
		return err
	}
	st.client.Close()
	webrtcvad.Free(*st.vadInst)

	return nil
}

func (st *SpeechToText) Start() error {
	go func() {
		for {
			select {
			case <-st.ctx.Done():
				return
			default:
				resp, err := (*st.stream).Recv()
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

				fmt.Println("\n\nInactive: ", time.Since(st.inactiveTime))
				st.onResponse(resp)
				st.lastRes = resp
			}
		}
	}()

	return nil
}

func (st *SpeechToText) Stop() {
	st.ctx.Done()
}

func (st *SpeechToText) OnInput(input []int16) error {
	// buf := make([]byte, SampleRate/1000*FrameDuration*BitDepth/8) //1024)
	frame, err := util.Int16ToBytesBinary(input, binary.LittleEndian)
	if err != nil {
		return err
	}

	frameActive, err := webrtcvad.Process(*st.vadInst, vars.SampleRate, frame, 16000/1000*20)
	if err != nil {
		return err
	}

	if frameActive {
		st.inactiveTime = time.Now()
	}

	if frameActive || time.Since(st.inactiveTime) < vars.VadInactiveTimeout*time.Millisecond {
		frameActive = true
	}

	if !frameActive && time.Since(st.inactiveTime) > time.Duration(vars.VadFinishTalkingTimeout)*time.Millisecond {
		fmt.Println("Finish talking!!!!!!!!!!!!")
	}

	if !frameActive {
		return nil
	}

	if err := (*st.stream).Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
			AudioContent: frame,
		},
	}); err != nil {
		return err
	}

	return nil
}
