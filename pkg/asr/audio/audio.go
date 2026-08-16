// Package audio 提供 ASR 场景下的音频格式工具函数。
//
// ASR 供应商对音频格式要求不一：讯飞 IAT 要求 raw PCM，FunASR 的
// ffmpeg 加载器需要带头的 WAV。本包提供 PCM/WAV 互转与格式探测，
// 供各供应商实现与上层业务复用，避免重复实现。
package audio

import "encoding/binary"

// DefaultSampleRate 是 ASR 常用采样率（16kHz）。
const DefaultSampleRate = 16000

// PCMToWAV 将 raw PCM（16-bit little-endian）封装为 WAV（44 字节 RIFF 头）。
//
// 参数：
//   - pcm:        原始 PCM 样本数据
//   - sampleRate: 采样率（<=0 时回退 DefaultSampleRate）
//   - channels:   声道数（<=0 时回退 1）
//   - bitsPerSample: 位深（<=0 时回退 16）
func PCMToWAV(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	if sampleRate <= 0 {
		sampleRate = DefaultSampleRate
	}
	if channels <= 0 {
		channels = 1
	}
	if bitsPerSample <= 0 {
		bitsPerSample = 16
	}

	const headerSize = 44
	bytesPerSample := bitsPerSample / 8
	dataLen := len(pcm)
	wav := make([]byte, headerSize+dataLen)

	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(36+dataLen))
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)                                         // fmt 块大小
	binary.LittleEndian.PutUint16(wav[20:22], 1)                                          // PCM
	binary.LittleEndian.PutUint16(wav[22:24], uint16(channels))                           // 声道数
	binary.LittleEndian.PutUint32(wav[24:28], uint32(sampleRate))                         // 采样率
	binary.LittleEndian.PutUint32(wav[28:32], uint32(sampleRate*bytesPerSample*channels)) // 字节率
	binary.LittleEndian.PutUint16(wav[32:34], uint16(bytesPerSample*channels))            // 块对齐
	binary.LittleEndian.PutUint16(wav[34:36], uint16(bitsPerSample))                      // 位深
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(dataLen))
	copy(wav[44:], pcm)
	return wav
}

// WAVToPCM 从 WAV 中提取 PCM 数据（跳过 RIFF 头）。
// 若 audio 不是 WAV（无 RIFF/WAVE 头），原样返回。
func WAVToPCM(audio []byte) []byte {
	if IsWAV(audio) {
		return audio[44:]
	}
	return audio
}

// IsWAV 判断音频是否为 RIFF/WAVE 格式（至少含 44 字节头）。
func IsWAV(audio []byte) bool {
	return len(audio) > 44 && string(audio[0:4]) == "RIFF" && string(audio[8:12]) == "WAVE"
}
