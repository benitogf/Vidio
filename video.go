package vidio

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

type Video struct {
	filename    string         // Video Filename.
	width       int            // Width of frames.
	height      int            // Height of frames.
	depth       int            // Depth of frames.
	bitrate     int            // Bitrate for video encoding.
	frames      int            // Total number of frames.
	duration    float64        // Duration of video in seconds.
	fps         float64        // Frames per second.
	codec       string         // Codec used for video encoding.
	audioCodec  string         // Codec used for audio encoding.
	framebuffer []byte         // Raw frame data.
	pipe        *io.ReadCloser // Stdout pipe for ffmpeg process.
	cmd         *exec.Cmd      // ffmpeg command.
}

func (video *Video) FileName() string {
	return video.filename
}

func (video *Video) Width() int {
	return video.width
}

func (video *Video) Height() int {
	return video.height
}

// Channels of video frames.
func (video *Video) Depth() int {
	return video.depth
}

// Bitrate of video.
func (video *Video) Bitrate() int {
	return video.bitrate
}

// Total number of frames in video.
func (video *Video) Frames() int {
	return video.frames
}

func (video *Video) Duration() float64 {
	return video.duration
}

func (video *Video) FPS() float64 {
	return video.fps
}

func (video *Video) Codec() string {
	return video.codec
}

func (video *Video) AudioCodec() string {
	return video.audioCodec
}

func (video *Video) FrameBuffer() []byte {
	return video.framebuffer
}

func (video *Video) SetFrameBuffer(buffer []byte) error {
	size := video.width * video.height * video.depth
	if len(buffer) < size {
		return fmt.Errorf("buffer size %d is smaller than frame size %d", len(buffer), size)
	}

	video.framebuffer = buffer
	return nil
}

// Creates a new Video struct.
// Uses ffprobe to get video information and fills in the Video struct with this data.
func NewVideo(filename string) (*Video, error) {
	if !exists(filename) {
		return nil, fmt.Errorf("video file %s does not exist", filename)
	}
	// Check if ffmpeg and ffprobe are installed on the users machine.
	if err := installed("ffmpeg"); err != nil {
		return nil, err
	}
	if err := installed("ffprobe"); err != nil {
		return nil, err
	}

	videoData, err := ffprobe(filename, "v")
	if err != nil {
		return nil, err
	}

	if len(videoData) == 0 {
		return nil, fmt.Errorf("no video data found in %s", filename)
	}

	audioData, err := ffprobe(filename, "a")
	if err != nil {
		return nil, err
	}

	video := &Video{filename: filename, depth: 3}

	video.addVideoData(videoData)
	if audioCodec, ok := audioData["codec_name"]; ok {
		video.audioCodec = audioCodec
	}

	return video, nil
}

// Adds Video data to the video struct from the ffprobe output.
func (video *Video) addVideoData(data map[string]string) {
	if width, ok := data["width"]; ok {
		video.width = int(parse(width))
	}
	if height, ok := data["height"]; ok {
		video.height = int(parse(height))
	}
	if duration, ok := data["duration"]; ok {
		video.duration = float64(parse(duration))
	}
	if frames, ok := data["nb_frames"]; ok {
		video.frames = int(parse(frames))
	}

	if fps, ok := data["r_frame_rate"]; ok {
		split := strings.Split(fps, "/")
		if len(split) == 2 && split[0] != "" && split[1] != "" {
			video.fps = parse(split[0]) / parse(split[1])
		}
	}

	if bitrate, ok := data["bit_rate"]; ok {
		video.bitrate = int(parse(bitrate))
	}
	if codec, ok := data["codec_name"]; ok {
		video.codec = codec
	}
}

// Once the user calls Read() for the first time on a Video struct,
// the ffmpeg command which is used to read the video is started.
func (video *Video) init() error {
	// If user exits with Ctrl+C, stop ffmpeg process.
	video.cleanup()
	// ffmpeg command to pipe video data to stdout in 8-bit RGB format.
	cmd := exec.Command(
		"ffmpeg",
		"-i", video.filename,
		"-f", "image2pipe",
		"-loglevel", "quiet",
		"-pix_fmt", "rgb24",
		"-vcodec", "rawvideo",
		"-",
	)

	video.cmd = cmd
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	video.pipe = &pipe
	if err := cmd.Start(); err != nil {
		return err
	}

	if video.framebuffer == nil {
		video.framebuffer = make([]byte, video.width*video.height*video.depth)
	}

	return nil
}

// Reads the next frame from the video and stores in the framebuffer.
// If the last frame has been read, returns false, otherwise true.
func (video *Video) Read() bool {
	// If cmd is nil, video reading has not been initialized.
	if video.cmd == nil {
		if err := video.init(); err != nil {
			return false
		}
	}
	total := 0
	for total < video.width*video.height*video.depth {
		n, err := (*video.pipe).Read(video.framebuffer[total:])
		if err == io.EOF {
			video.Close()
			return false
		}
		total += n
	}
	return true
}

// Closes the pipe and stops the ffmpeg process.
func (video *Video) Close() {
	if video.pipe != nil {
		(*video.pipe).Close()
	}
	if video.cmd != nil {
		video.cmd.Wait()
	}
}

// Stops the "cmd" process running when the user presses Ctrl+C.
// https://stackoverflow.com/questions/11268943/is-it-possible-to-capture-a-ctrlc-signal-and-run-a-cleanup-function-in-a-defe.
func (video *Video) cleanup() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if video.pipe != nil {
			(*video.pipe).Close()
		}
		if video.cmd != nil {
			video.cmd.Process.Kill()
		}
		os.Exit(1)
	}()
}
