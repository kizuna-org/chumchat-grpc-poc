package vars

const (
	VadMode       = 3
	SampleRate    = 16000
	BitDepth      = 16
	FrameDuration = 20
)

const (
	FramesPerBuffer = 1024
)

const (
	VadInactiveTimeout = 100 // 経ったらvadのactiveをfalseに ms
)

var (
	VadFinishTalkingTimeout = VadInactiveTimeout + 100 // 経ったら喋り終わった ms
)

const (
	Location       = "us-central1"
	ProjectID      = "chumchat"
	GenAIModelName = "gemini-1.5-flash"
)

const SystemPrompt = `
Please interact in English.
Please respond in 1-2 sentences.
`

const IsSkipWhenOutput = true

const SSTSameDistance = 5
