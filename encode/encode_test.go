package encode

// TODO: test v.Copy.
import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixValues(t *testing.T) {
	a := assert.New(t)
	v := Get(&VidOps{})
	// Test default values.
	a.False(v.SetAudio(""), "Wrong default 'audio' value!")
	a.EqualValues(DefaultProfile, v.SetProfile(""), "Wrong default 'profile' value!")
	a.EqualValues(DefaultLevel, v.SetLevel(""), "Wrong default 'level' value!")
	a.EqualValues(DefaultFrameHeight, v.SetHeight(""), "Wrong default 'height' value!")
	a.EqualValues(DefaultFrameWidth, v.SetWidth(""), "Wrong default 'width' value!")
	a.EqualValues(DefaultEncodeCRF, v.SetCRF(""), "Wrong default 'crf' value!")
	a.EqualValues(DefaultCaptureTime, v.SetTime(""), "Wrong default 'time' value!")
	a.EqualValues(DefaultFrameRate, v.SetRate(""), "Wrong default 'rate' value!")
	a.EqualValues(int64(DefaultCaptureSize), v.SetSize(""), "Wrong default 'size' value!")
	// Text max values.
	a.EqualValues(MaximumFrameSize, v.SetHeight("9000"), "Wrong maximum 'height' value!")
	a.EqualValues(MaximumFrameSize, v.SetWidth("9000"), "Wrong maximum 'width' value!")
	a.EqualValues(MaximumEncodeCRF, v.SetCRF("9000"), "Wrong maximum 'crf' value!")
	a.EqualValues(MaximumCaptureTime, v.SetTime("9000"), "Wrong maximum 'time' value!")
	a.EqualValues(MaximumFrameRate, v.SetRate("9000"), "Wrong maximum 'rate' value!")
	a.EqualValues(int64(MaximumCaptureSize), v.SetSize("999999999"), "Wrong maximum 'size' value!")
	// Text min values.
	a.EqualValues(MinimumFrameSize, v.SetHeight("1"), "Wrong minimum 'height' value!")
	a.EqualValues(MinimumFrameSize, v.SetWidth("1"), "Wrong minimum 'width' value!")
	a.EqualValues(MinimumEncodeCRF, v.SetCRF("1"), "Wrong minimum 'CRF' value!")
	a.EqualValues(MinimumFrameRate, v.SetRate("1"), "Wrong minimum 'rate' value!")
}

func TestGetVideo(t *testing.T) {
	a := assert.New(t)
	v := Get(&VidOps{Encoder: "/bin/echo"})
	cmd, out, err := v.GetVideo("ID123", "INPUT", "OUTPUT", "TITLE")
	a.Nil(err, "/bin/echo returned an error. Something may be wrong with your environment.")
	// Make sure the produced command has all the expected values.
	a.Contains(cmd, "-an", "Audio may not be correctly disabled.")
	a.Contains(cmd, "-rtsp_transport tcp -i INPUT", "INPUT value appears to be missing, or rtsp transport is out of order")
	a.Contains(cmd, "-metadata title=\"TITLE\"", "TITLE value appears to be missing.")
	a.Contains(cmd, fmt.Sprintf("-vcodec libx264 -profile:v %v -level %v", DefaultProfile, DefaultLevel), "Level or Profile are missing or out of order.")
	a.Contains(cmd, fmt.Sprintf("-crf %d", DefaultEncodeCRF), "CRF value is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-t %d", DefaultCaptureTime), "Capture Time value is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-s %dx%d", DefaultFrameWidth, DefaultFrameHeight), "Framesize is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-r %d", DefaultFrameRate), "Frame Rate value is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-fs %d", DefaultCaptureSize), "Size value is missing or malformed.")
	a.True(strings.HasPrefix(cmd, "/bin/echo"), "The command does not - but should - begin with the Encoder value.")
	a.True(strings.HasSuffix(cmd, "OUTPUT"), "The command does not - but should - end with the output value.")
	a.EqualValues(cmd+"\n", "/bin/echo "+out, "Somehow the wrong value was echo'd.")
	// Make sure audio can be turned on.
	v = Get(&VidOps{Encoder: "/bin/echo", Audio: true})
	cmd, _, err = v.GetVideo("ID123", "INPUT", "OUTPUT", "TITLE")
	a.Nil(err, "/bin/echo returned an error. Something may be wrong with your environment.")
	a.Contains(cmd, "-c:a copy", "Audio may not be correctly enabled.")
}
