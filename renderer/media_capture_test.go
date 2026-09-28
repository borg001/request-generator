package renderer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validMediaCapture() *MediaCaptureConfig {
	return &MediaCaptureConfig{
		Kind:        MediaCaptureKindPhoto,
		Frame:       MediaCaptureFrameFace,
		Facing:      MediaCaptureFacingUser,
		OpenLabel:   "capture.open",
		Title:       "capture.title",
		Hint:        "capture.hint",
		ShootLabel:  "capture.shoot",
		RetakeLabel: "capture.retake",
		UseLabel:    "capture.use",
		CloseLabel:  "capture.close",
		PhoneLabel:  "capture.phone",
		PhoneTitle:  "capture.phone_title",
		PhoneText:   "capture.phone_text",
	}
}

func TestMediaCaptureConfigValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*MediaCaptureConfig)
		err    string
	}{
		{name: "valid photo"},
		{name: "valid video", mutate: func(config *MediaCaptureConfig) {
			config.Kind, config.StopLabel, config.MinDurationSeconds, config.MaxDurationSeconds = MediaCaptureKindVideo, "capture.stop", 10, 60
		}},
		{name: "full length with a timer", mutate: func(config *MediaCaptureConfig) {
			config.Frame, config.Facing, config.TimerSeconds = MediaCaptureFrameBody, MediaCaptureFacingEnvironment, 10
		}},
		{name: "unsupported kind", mutate: func(config *MediaCaptureConfig) { config.Kind = "audio" }, err: `renderer.MediaCaptureConfig: unsupported kind "audio"`},
		{name: "unsupported frame", mutate: func(config *MediaCaptureConfig) { config.Frame = "hand" }, err: `renderer.MediaCaptureConfig: unsupported frame "hand"`},
		{name: "unsupported facing", mutate: func(config *MediaCaptureConfig) { config.Facing = "left" }, err: `renderer.MediaCaptureConfig: unsupported facing "left"`},
		{name: "negative timer", mutate: func(config *MediaCaptureConfig) { config.TimerSeconds = -1 }, err: "renderer.MediaCaptureConfig: durations cannot be negative"},
		{name: "min over max", mutate: func(config *MediaCaptureConfig) { config.MinDurationSeconds, config.MaxDurationSeconds = 20, 10 }, err: "renderer.MediaCaptureConfig: min duration exceeds max duration"},
		{name: "missing use label", mutate: func(config *MediaCaptureConfig) { config.UseLabel = " " }, err: "renderer.MediaCaptureConfig: use label is required"},
		{name: "video without a stop label", mutate: func(config *MediaCaptureConfig) { config.Kind = MediaCaptureKindVideo }, err: "renderer.MediaCaptureConfig: stop label is required for a video"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			capture := validMediaCapture()
			if test.mutate != nil {
				test.mutate(capture)
			}
			if test.err == "" {
				require.NoError(t, capture.Validate())
				return
			}
			require.EqualError(t, capture.Validate(), test.err)
		})
	}
}

// A field's capture is checked with the rest of its media, a copy of the media
// does not share it, and its words reach the reader translated.
func TestFieldMediaCarriesItsCapture(t *testing.T) {
	media := &FieldMediaConfig{Capture: validMediaCapture()}
	require.NoError(t, media.Validate())
	media.Capture.Frame = "hand"
	require.EqualError(t, media.Validate(), `renderer.MediaCaptureConfig: unsupported frame "hand"`)

	media.Capture = validMediaCapture()
	copied := CloneFieldMediaConfig(media)
	copied.Capture.Title = "changed"
	require.Equal(t, "capture.title", media.Capture.Title)

	localized := LocalizeFieldMedia(media, func(value, _ string) string { return "T:" + value })
	require.Equal(t, "T:capture.open", localized.Capture.OpenLabel)
	require.Equal(t, "T:capture.phone_text", localized.Capture.PhoneText)
	require.Equal(t, "capture.open", media.Capture.OpenLabel)
}

// A video led through steps says what each one shows and for how long; a copy
// of the media does not share them, nor what each step asks the person to do,
// and their words reach the reader translated.
func TestMediaCaptureSteps(t *testing.T) {
	capture := validMediaCapture()
	capture.Kind, capture.StopLabel, capture.StepLabel = MediaCaptureKindVideo, "capture.stop", "capture.step"
	capture.Steps = []MediaCaptureStep{
		{Frame: MediaCaptureFrameFace, Hint: "capture.close", Seconds: 4},
		{Frame: MediaCaptureFrameBody, Props: []MediaCaptureProp{MediaCapturePropSign}, Hint: "capture.back", Seconds: 5},
		{Frame: MediaCaptureFrameBody, Props: []MediaCaptureProp{MediaCapturePropSign, MediaCapturePropSpeech}, Hint: "capture.nick", Seconds: 5},
	}
	require.NoError(t, capture.Validate())

	media := &FieldMediaConfig{Capture: capture}
	copied := CloneFieldMediaConfig(media)
	copied.Capture.Steps[0].Hint = "changed"
	copied.Capture.Steps[2].Props[1] = MediaCapturePropSign
	require.Equal(t, "capture.close", media.Capture.Steps[0].Hint)
	require.Equal(t, []MediaCaptureProp{MediaCapturePropSign, MediaCapturePropSpeech}, media.Capture.Steps[2].Props)
	require.Nil(t, copied.Capture.Steps[0].Props)

	localized := LocalizeFieldMedia(media, func(value, _ string) string { return "T:" + value })
	require.Equal(t, "T:capture.back", localized.Capture.Steps[1].Hint)
	require.Equal(t, "T:capture.step", localized.Capture.StepLabel)
	media.Capture.NextStepLabel, media.Capture.DoneTitle, media.Capture.DoneText = "capture.next", "capture.done", "capture.done_text"
	localized = LocalizeFieldMedia(media, func(value, _ string) string { return "T:" + value })
	require.Equal(t, "T:capture.next", localized.Capture.NextStepLabel)
	require.Equal(t, "T:capture.done", localized.Capture.DoneTitle)
	require.Equal(t, "T:capture.done_text", localized.Capture.DoneText)
	require.Equal(t, "capture.done", media.Capture.DoneTitle)
	media.Capture.PermissionLabel, media.Capture.RetryLabel = "capture.allow", "capture.retry"
	localized = LocalizeFieldMedia(media, func(value, _ string) string { return "T:" + value })
	require.Equal(t, "T:capture.allow", localized.Capture.PermissionLabel)
	require.Equal(t, "T:capture.retry", localized.Capture.RetryLabel)
	require.Equal(t, "capture.back", media.Capture.Steps[1].Hint)

	capture.Steps[2].Props = []MediaCaptureProp{MediaCapturePropSign, "dance"}
	require.EqualError(t, capture.Validate(), `renderer.MediaCaptureConfig: step 3 has unsupported prop "dance"`)
	capture.Steps[2].Props, capture.Steps[2].Seconds = []MediaCaptureProp{MediaCapturePropSpeech}, 0
	require.EqualError(t, capture.Validate(), "renderer.MediaCaptureConfig: step 3 needs a hint and its seconds")
}
