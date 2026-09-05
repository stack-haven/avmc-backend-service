// Package audio · audio_test.go
// 音频格式探测 + 转码（M9 修复：mp3 → wav）。
package audio

import "testing"

func TestIsMP3(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  bool
	}{
		{
			name:  "ID3v2 header",
			input: []byte("ID3\x04\x00\x00\x00\x00\x00\x00"),
			want:  true,
		},
		{
			name:  "MPEG frame sync 0xFF 0xFB",
			input: []byte{0xFF, 0xFB, 0x90, 0x00},
			want:  true,
		},
		{
			name:  "MPEG frame sync 0xFF 0xF3",
			input: []byte{0xFF, 0xF3, 0x88, 0xC4},
			want:  true,
		},
		{
			name: "ID3v1 tag (tail)",
			input: func() []byte {
				b := make([]byte, 128+50) // 50 bytes mp3 + 128 bytes ID3v1
				copy(b[len(b)-128:], []byte("TAG\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
				return b
			}(),
			want: true,
		},
		{
			name:  "WAV header (RIFF)",
			input: []byte("RIFF\x00\x00\x00\x00WAVEfmt "),
			want:  false,
		},
		{
			name:  "raw PCM",
			input: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
			want:  false,
		},
		{
			name:  "empty",
			input: []byte{},
			want:  false,
		},
		{
			name:  "too short",
			input: []byte{0xFF, 0xFB},
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsMP3(tc.input); got != tc.want {
				t.Errorf("IsMP3 = %v, want %v (len=%d)", got, tc.want, len(tc.input))
				if len(tc.input) >= 3 {
					t.Logf("first 3: %q", tc.input[:3])
				}
				if len(tc.input) >= 128 {
					t.Logf("tail-128..tail-125: %q", tc.input[len(tc.input)-128:len(tc.input)-125])
				}
			}
		})
	}
}

func TestTranscodeToWAV_NotMP3(t *testing.T) {
	_, err := TranscodeToWAV([]byte("RIFF\x00\x00\x00\x00WAVEfmt "))
	if err == nil {
		t.Error("expected error for non-MP3 input")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}