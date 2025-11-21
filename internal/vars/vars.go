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
	VadInactiveTimeout      = 50                       // 経ったらvadのactiveをfalseに ms
	VadFinishTalkingTimeout = VadInactiveTimeout + 100 // 経ったら喋り終わった ms
	VadActiveTime           = 50                       // 経ったらvadのactiveをtrueに ms
)

const (
	Location       = "us-central1"
	ProjectID      = "chumchat"
	GenAIModelName = "gemini-2.0-flash-lite-preview-02-05"
)

const SystemPrompt = `
Please interact in English.
Please respond in 1-2 sentences.
`

const (
	IsSkipWhenOutput = true
	OutputSkipTime   = 1000 // ms
)

const SSTSameDistance = 5
