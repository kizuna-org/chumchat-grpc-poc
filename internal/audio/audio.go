package audio

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/kizuna-org/chumchat-grpc-poc/internal/vars"
)

type AudioStream struct {
	stream *portaudio.Stream
	input  []int16
	output []int16

	ctx               context.Context
	GlobalOutputMutex sync.Mutex
	GlobalOutput      []int16

	onInput func([]int16) error
}

func (as *AudioStream) Output(output []int16) {
	as.GlobalOutputMutex.Lock()
	defer as.GlobalOutputMutex.Unlock()
	as.GlobalOutput = output
}

func (as *AudioStream) Close() error {
	portaudio.Terminate()

	return as.stream.Close()
}

func (as *AudioStream) Start() error {
	portaudio.Initialize()
	go playAudio(as)
	go func() {
		for {
			select {
			case <-as.ctx.Done():
				return
			default:
				err := as.onInput(as.input)
				if err != nil {
					log.Printf("onInput failed: %v", err)
				}
			}
		}
	}()

	return as.stream.Start()
}

func (as *AudioStream) Stop() error {
	as.ctx.Done()

	return as.stream.Stop()
}

func playAudio(as *AudioStream) {
	for {
		select {
		case <-as.ctx.Done():
			return
		default:
			as.GlobalOutputMutex.Lock()
			if len(as.GlobalOutput) == 0 {
				output := make([]int16, vars.FramesPerBuffer)
				copy(as.output, output)
				err := as.stream.Write()
				if err != nil {
					log.Printf("stream.Write() failed: %v", err)
				}

				as.GlobalOutputMutex.Unlock()
				time.Sleep(time.Millisecond * 10)
				continue
			}

			copyLength := vars.FramesPerBuffer
			if len(as.GlobalOutput) < vars.FramesPerBuffer {
				copyLength = len(as.GlobalOutput)
			}

			copiedData := make([]int16, vars.FramesPerBuffer)
			copy(copiedData, as.GlobalOutput[:copyLength])

			copy(as.output, copiedData)

			as.GlobalOutput = as.GlobalOutput[copyLength:]
			as.GlobalOutputMutex.Unlock()

			fmt.Printf("Copied %d elements to as.output. Remaining globalOutput: %d\n", copyLength, len(as.GlobalOutput))

			err := as.stream.Write()
			if err != nil {
				log.Printf("stream.Write() failed: %v", err)
			}
		}
	}
}

func NewAudioStream(onInput func([]int16) error) (*AudioStream, error) {
	input := make([]int16, vars.FramesPerBuffer)
	output := make([]int16, vars.FramesPerBuffer)

	stream, err := portaudio.OpenDefaultStream(1, 1, vars.SampleRate, vars.FramesPerBuffer, input, output)
	if err != nil {
		return nil, err
	}

	return &AudioStream{
		stream: stream,
		input:  input,
		output: output,
		ctx:    context.Background(),
	}, nil
}
