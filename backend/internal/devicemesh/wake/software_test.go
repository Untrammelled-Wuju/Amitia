package wake

import (
	"testing"
)

func TestSoftwareDetectorDetectsRisingAudio(t *testing.T) {
	detector := NewSoftwareDetector()
	if err := detector.Load(t.Context(), WakeDetectorConfig{Enabled: true, Phrases: []WakePhrase{{ID: "wake", DisplayText: "你好"}}, Threshold: 0.01}); err != nil {
		t.Fatal(err)
	}
	amplitude := []int16{100, 1000, 12000}
	for _, value := range amplitude {
		pcm := []byte{byte(uint16(value)), byte(uint16(value) >> 8)}
		result, err := detector.Process(&VoiceAudioFrame{PCM: pcm, SampleRate: CanonicalSampleRate, Channels: CanonicalChannels, Encoding: CanonicalEncoding})
		if err != nil {
			t.Fatal(err)
		}
		if value == 12000 && (!result.Detected || result.PhraseID != "wake") {
			t.Fatalf("rising audio was not detected: %+v", result)
		}
	}
}
