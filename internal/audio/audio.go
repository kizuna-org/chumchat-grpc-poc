package audio

import (
	"context"
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
	globalOutputMutex sync.Mutex
	globalOutput      []int16

	onInput func([]int16) error

	lastOutput time.Time
}

func (as *AudioStream) Output(output []int16) {
	as.globalOutputMutex.Lock()
	defer as.globalOutputMutex.Unlock()
	as.globalOutput = output
}

func (as *AudioStream) Close() error {
	portaudio.Terminate()

	return as.stream.Close()
}

func (as *AudioStream) Start() error {
	go playAudio(as)
	go func() {
		for {
			select {
			case <-as.ctx.Done():
				return
			default:
				err := as.stream.Read()
				if err != nil {
					log.Printf("stream.Read() failed: %v", err)
				}

				if vars.IsSkipWhenOutput && len(as.globalOutput) > 0 {
					continue
				}

				if vars.IsSkipWhenOutput && time.Since(as.lastOutput) < time.Millisecond*vars.OutputSkipTime {
					continue
				}

				if as.onInput != nil {
					err := as.onInput(as.input)
					if err != nil {
						log.Printf("onInput failed: %v", err)
					}
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
			as.globalOutputMutex.Lock()
			if len(as.globalOutput) == 0 {
				output := make([]int16, vars.FramesPerBuffer)
				copy(as.output, output)
				err := as.stream.Write()
				if err != nil {
					log.Printf("stream.Write() failed: %v", err)
				}

				as.globalOutputMutex.Unlock()
				time.Sleep(time.Millisecond * 10)
				continue
			}

			copyLength := vars.FramesPerBuffer
			if len(as.globalOutput) < vars.FramesPerBuffer {
				copyLength = len(as.globalOutput)
			}

			copiedData := make([]int16, vars.FramesPerBuffer)
			copy(copiedData, as.globalOutput[:copyLength])

			copy(as.output, copiedData)

			as.globalOutput = as.globalOutput[copyLength:]
			as.globalOutputMutex.Unlock()

			// fmt.Printf("Copied %d elements to as.output. Remaining globalOutput: %d\n", copyLength, len(as.globalOutput))

			err := as.stream.Write()
			if err != nil {
				log.Printf("stream.Write() failed: %v", err)
			}

			as.lastOutput = time.Now()

			time.Sleep(time.Duration(vars.FramesPerBuffer) * time.Second / time.Duration(vars.SampleRate))
		}
	}
}

func NewAudioStream() (*AudioStream, error) {
	portaudio.Initialize()

	input := make([]int16, vars.FramesPerBuffer)
	output := make([]int16, vars.FramesPerBuffer)

	stream, err := portaudio.OpenDefaultStream(1, 1, vars.SampleRate, vars.FramesPerBuffer, &input, &output)
	if err != nil {
		return nil, err
	}

	return &AudioStream{
		stream:            stream,
		input:             input,
		output:            output,
		ctx:               context.Background(),
		onInput:           nil,
		globalOutputMutex: sync.Mutex{},
		globalOutput:      []int16{},
	}, nil
}

func (as *AudioStream) SetOnInput(onInput func([]int16) error) {
	as.onInput = onInput
}
