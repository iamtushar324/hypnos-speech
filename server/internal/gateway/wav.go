package gateway

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type wavParts struct {
	format []byte
	data   []byte
}

func parseWAV(b []byte) (wavParts, error) {
	return parseWAVPayload(b, false)
}

func parseWAVPayload(b []byte, allowEmpty bool) (wavParts, error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return wavParts{}, errors.New("provider did not return a WAV file")
	}
	riffSize := binary.LittleEndian.Uint32(b[4:8])
	if riffSize != ^uint32(0) && uint64(riffSize)+8 > uint64(len(b)) {
		return wavParts{}, errors.New("truncated WAV container")
	}
	var out wavParts
	dataSeen := false
	for off := 12; off+8 <= len(b); {
		declared := binary.LittleEndian.Uint32(b[off+4 : off+8])
		start := off + 8
		end := len(b)
		if declared != ^uint32(0) {
			if uint64(start)+uint64(declared) > uint64(len(b)) {
				return wavParts{}, errors.New("truncated WAV chunk")
			}
			end = start + int(declared)
		} else if string(b[off:off+4]) != "data" {
			return wavParts{}, errors.New("invalid streamed WAV chunk size")
		}
		if end < start {
			return wavParts{}, errors.New("truncated WAV chunk")
		}
		switch string(b[off : off+4]) {
		case "fmt ":
			out.format = append([]byte(nil), b[start:end]...)
		case "data":
			if dataSeen && allowEmpty {
				return wavParts{}, errors.New("multiple WAV audio chunks")
			}
			dataSeen = true
			out.data = append(out.data, b[start:end]...)
		}
		off = end
		if declared != ^uint32(0) && declared%2 == 1 {
			off++
		}
	}
	if len(out.format) < 16 || (!allowEmpty && len(out.data) == 0) || !dataSeen {
		return wavParts{}, errors.New("WAV is missing format or audio data")
	}
	blockAlign := int(binary.LittleEndian.Uint16(out.format[12:14]))
	if blockAlign <= 0 || len(out.data)%blockAlign != 0 {
		return wavParts{}, errors.New("WAV contains a partial audio frame")
	}
	return out, nil
}

func concatenateWAV(waves [][]byte) ([]byte, error) {
	if len(waves) == 0 {
		return nil, errors.New("no WAV chunks")
	}
	var format, data []byte
	for _, w := range waves {
		p, err := parseWAV(w)
		if err != nil {
			return nil, err
		}
		if format == nil {
			format = p.format
		} else if !bytes.Equal(format, p.format) {
			return nil, errors.New("provider WAV chunks use different formats")
		}
		data = append(data, p.data...)
	}
	formatPad := len(format) % 2
	total := 4 + (8 + len(format) + formatPad) + (8 + len(data))
	if uint64(total) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("combined WAV is too large")
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(total))
	out.WriteString("WAVEfmt ")
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(format)))
	out.Write(format)
	if len(format)%2 == 1 {
		out.WriteByte(0)
	}
	out.WriteString("data")
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(data)))
	out.Write(data)
	return out.Bytes(), nil
}
