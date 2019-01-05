package encode

/* Encode videos from RTSP IP camera URLs using FFMPEG */

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// Default, Maximum and Minimum Values
const (
	DefaultFrameRate   = 5
	MinimumFrameRate   = 2
	MaximumFrameRate   = 30
	DefaultFrameHeight = 720
	DefaultFrameWidth  = 1280
	MinimumFrameSize   = 100
	MaximumFrameSize   = 4000
	DefaultEncodeCRF   = 21
	MinimumEncodeCRF   = 16
	MaximumEncodeCRF   = 30
	DefaultCaptureTime = 15
	MaximumCaptureTime = 60
	DefaultCaptureSize = 2500000
	MaximumCaptureSize = 22000000
	DefaultFFmpegPath  = "/usr/local/bin/ffmpeg"
	DefaultProfile     = "main"
	DefaultLevel       = "3.0"
)

// Encoder provides an inteface to mock.
type Encoder interface {
	GetVideo(id, input, output, title string) (cmd string, cmdout string, err error)
	SetAudio(audio string) (value bool)
	SetRate(rate string) (value int)
	SetLevel(level string) (value string)
	SetWidth(width string) (value int)
	SetHeight(height string) (value int)
	SetCRF(crf string) (value int)
	SetTime(seconds string) (value int)
	SetSize(size string) (value int64)
	SetProfile(profile string) (value string)
}

// VidOps defines how to ffmpeg shall transcode a stream.
type VidOps struct {
	Encoder string // "/usr/local/bin/ffmpeg"
	Level   string // 3.0, 3.1 ..
	Width   int    // 1920
	Height  int    // 1080
	CRF     int    // 24
	Time    int    // 15 (seconds)
	Audio   bool   // include audio?
	Rate    int    // framerate (5-20)
	Size    int64  // max file size (always goes over). use 2000000 for 2.5MB
	Prof    string // main, high, baseline
	Copy    bool   // Copy original stream, rather than transcode.
}

// Get an encoder interface.
func Get(v *VidOps) Encoder {
	if v.Encoder == "" {
		v.Encoder = DefaultFFmpegPath
	}
	v.SetLevel(v.Level)
	v.SetProfile(v.Prof)
	v.fixValues()
	return v
}

// SetAudio turns audio on or off based on a string value.
func (v *VidOps) SetAudio(audio string) bool {
	v.Audio, _ = strconv.ParseBool(audio)
	return v.Audio
}

// SetLevel sets the h264 transcode level.
func (v *VidOps) SetLevel(level string) string {
	if v.Level = level; level != "3.0" && level != "3.1" && level != "4.0" && level != "4.1" && level != "4.2" {
		v.Level = DefaultLevel
	}
	return v.Level
}

// SetProfile sets the h264 transcode profile.
func (v *VidOps) SetProfile(profile string) string {
	if v.Prof = profile; v.Prof != "main" && v.Prof != "baseline" && v.Prof != "high" {
		v.Prof = DefaultProfile
	}
	return v.Prof
}

// SetWidth sets the transcode frame width.
func (v *VidOps) SetWidth(width string) int {
	v.Width, _ = strconv.Atoi(width)
	v.fixValues()
	return v.Width
}

// SetHeight sets the transcode frame width.
func (v *VidOps) SetHeight(height string) int {
	v.Height, _ = strconv.Atoi(height)
	v.fixValues()
	return v.Height
}

// SetCRF sets the h264 transcode CRF value.
func (v *VidOps) SetCRF(crf string) int {
	v.CRF, _ = strconv.Atoi(crf)
	v.fixValues()
	return v.CRF
}

// SetTime sets the maximum transcode duration.
func (v *VidOps) SetTime(seconds string) int {
	v.Time, _ = strconv.Atoi(seconds)
	v.fixValues()
	return v.Time
}

// SetRate sets the transcode framerate.
func (v *VidOps) SetRate(rate string) int {
	v.Rate, _ = strconv.Atoi(rate)
	v.fixValues()
	return v.Rate
}

// SetSize sets the maximum transcode file size.
func (v *VidOps) SetSize(size string) int64 {
	v.Size, _ = strconv.ParseInt(size, 10, 64)
	v.fixValues()
	return v.Size
}

// GetVideo retreives video from an input and saves it to an output.
func (v *VidOps) GetVideo(id, input, output, title string) (string, string, error) {
	arg := []string{
		v.Encoder,
		"-rtsp_transport", "tcp",
		"-i", input,
		"-metadata", `title="` + title + `"`,
		"-fs", strconv.FormatInt(v.Size, 10),
		"-t", strconv.Itoa(v.Time),
		"-y", "-map", "0",
	}
	if !v.Copy {
		arg = append(arg, "-vcodec", "libx264",
			"-profile:v", v.Prof,
			"-level", v.Level,
			"-pix_fmt", "yuv420p",
			"-movflags", "faststart",
			"-s", strconv.Itoa(v.Width)+"x"+strconv.Itoa(v.Height),
			"-preset", "superfast",
			"-crf", strconv.Itoa(v.CRF),
			"-r", strconv.Itoa(v.Rate),
		)
	} else {
		arg = append(arg, "-c", "copy")
	}
	if !v.Audio {
		arg = append(arg, "-an")
	} else {
		arg = append(arg, "-c:a", "copy")
	}
	arg = append(arg, output)
	var out bytes.Buffer
	cmd := exec.Command(arg[0], arg[1:]...)
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return strings.Join(arg, " "), out.String(), err
}

// fixValues makes sure video request values are sane.
func (v *VidOps) fixValues() {
	if v.Height == 0 {
		v.Height = DefaultFrameHeight
	} else if v.Height > MaximumFrameSize {
		v.Height = MaximumFrameSize
	} else if v.Height < MinimumFrameSize {
		v.Height = MinimumFrameSize
	}

	if v.Width == 0 {
		v.Width = DefaultFrameWidth
	} else if v.Width > MaximumFrameSize {
		v.Width = MaximumFrameSize
	} else if v.Width < MinimumFrameSize {
		v.Width = MinimumFrameSize
	}

	if v.CRF == 0 {
		v.CRF = DefaultEncodeCRF
	} else if v.CRF < MinimumEncodeCRF {
		v.CRF = MinimumEncodeCRF
	} else if v.CRF > MaximumEncodeCRF {
		v.CRF = MaximumEncodeCRF
	}

	if v.Rate == 0 {
		v.Rate = DefaultFrameRate
	} else if v.Rate < MinimumFrameRate {
		v.Rate = MinimumFrameRate
	} else if v.Rate > MaximumFrameRate {
		v.Rate = MaximumFrameRate
	}

	// No minimums.
	if v.Time == 0 {
		v.Time = DefaultCaptureTime
	} else if v.Time > MaximumCaptureTime {
		v.Time = MaximumCaptureTime
	}

	if v.Size == 0 {
		v.Size = DefaultCaptureSize
	} else if v.Size > MaximumCaptureSize {
		v.Size = MaximumCaptureSize
	}
}
