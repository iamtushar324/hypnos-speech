package gateway

import (
	"encoding/binary"
	"math"
	"net/http"
)

// silentWAV is a conservative signal-presence gate, not a speech recognizer.
// Require 40 consecutive ms above -55 dBFS RMS (measured in 20 ms windows), excluding DC.
// A short click on opening/closing the mic cannot trigger transcription; a short
// quiet word is sufficient even inside a long recording. Never trim speech audio.
// Unknown/compressed/malformed formats pass through to the existing provider.
func silentWAV(audio []byte) bool {
	wav, err := parseWAVPayload(audio, true)
	if err != nil {
		return false
	}
	format := binary.LittleEndian.Uint16(wav.format[0:2])
	channels := int(binary.LittleEndian.Uint16(wav.format[2:4]))
	rate := int(binary.LittleEndian.Uint32(wav.format[4:8]))
	align := int(binary.LittleEndian.Uint16(wav.format[12:14]))
	bits := int(binary.LittleEndian.Uint16(wav.format[14:16]))
	if channels < 1 || channels > 8 || rate < 8000 || rate > 192000 {
		return false
	}
	if format == 1 {
		if bits != 8 && bits != 16 && bits != 24 && bits != 32 {
			return false
		}
	} else if format == 3 {
		if bits != 32 && bits != 64 {
			return false
		}
	} else {
		return false
	}
	width := bits / 8
	if align != channels*width {
		return false
	}
	frames := len(wav.data) / align
	window := rate / 50
	activeFrames := 0
	const minimumPower = 0.0000031622776601683795 // 10^(-55/10)
	for start := 0; start < frames; start += window {
		end := min(start+window, frames)
		active := false
		// Evaluate channels independently: opposite-phase stereo must not cancel.
		for channel := 0; channel < channels; channel++ {
			sum, squares := 0.0, 0.0
			for frame := start; frame < end; frame++ {
				offset := frame*align + channel*width
				b := wav.data[offset : offset+width]
				var value float64
				if format == 3 {
					if bits == 32 {
						value = float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
					} else {
						value = math.Float64frombits(binary.LittleEndian.Uint64(b))
					}
					if math.IsNaN(value) || math.IsInf(value, 0) {
						return false
					}
				} else {
					switch bits {
					case 8:
						value = float64(int(b[0])-128) / 128
					case 16:
						value = float64(int16(binary.LittleEndian.Uint16(b))) / 32768
					case 24:
						n := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16
						value = float64(n<<8>>8) / 8388608
					case 32:
						value = float64(int32(binary.LittleEndian.Uint32(b))) / 2147483648
					}
				}
				sum += value
				squares += value * value
			}
			count := float64(end - start)
			power := squares/count - (sum/count)*(sum/count)
			if power >= minimumPower {
				active = true
			}
		}
		if active {
			activeFrames += end - start
		} else {
			activeFrames = 0
		}
		if activeFrames >= rate*40/1000 {
			return false
		}
	}
	return true
}

func writeNoSpeech(w http.ResponseWriter, format string) {
	w.Header().Set("X-Speech-No-Speech", "true")
	if format == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": "", "no_speech": true, "cleanup_status": "skipped"})
}
