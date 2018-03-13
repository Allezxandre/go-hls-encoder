package suggest

import (
	"errors"
	"github.com/allezxandre/go-hls-encoder/probe"
	"log"
	"strconv"
	"strings"
)

//
//
// Find the masterVideo stream.  At this point it's usually just
// the only video stream, but may need to add code here for the
// situation where we have more than one video
//
//
func masterVideo(fileStreams []*probe.ProbeStream) (streamIndex int, err error) {
	for _, stream := range fileStreams {
		if stream.CodecType == "video" {
			streamIndex := stream.Index
			return streamIndex, nil
		}
	}
	err = errors.New("could not find a video stream to use as master")
	return
}

type VideoVariant struct {
	MapInput string  // The map value: in the form of $input:$stream
	Codec    string  // Codec to use, or "copy". Required.
	CRF      *int    // Optional. CRF Value.
	Profile  *string // Optional
	Level    *string // Required if `Profile` is provided.
	Bitrate  *string // Optional
	// Associated Media
	AudioGroup    *string // Optional Audio Group
	SubtitleGroup *string // Optional Subtitle Group
	// M3U8 Playlist options
	Resolution       string // Resolution for variant in M3U8 playlist
	Bandwidth        string
	ResolutionHeight *int // Optional. To use as -filter:v scale="trunc(oh*a/2)*2:HEIGHT"
}

func SuggestVideoVariants(probeDataInputs []*probe.ProbeData) (variants []VideoVariant) {
	for inputIndex, probeData := range probeDataInputs { // Loop through inputs
		if masterVideoIndex, err := masterVideo(probeData.Streams); err == nil {
			// Found a video in this input
			videoStream := probeData.Streams[masterVideoIndex]
			bandwidth := 7000000 // FIXME: Handle unknown bandwidth
			if bandwitdhInt, err := strconv.Atoi(videoStream.BitRate); err == nil {
				bandwidth = bandwitdhInt
			}
			// Match codec
			switch videoStream.CodecName {
			case "h264":
				// Only one variant: copy
				variants = append(variants, VideoVariant{
					MapInput:   strconv.Itoa(inputIndex) + ":" + strconv.Itoa(masterVideoIndex),
					Codec:      "copy",
					Resolution: strconv.Itoa(videoStream.Width) + "x" + strconv.Itoa(videoStream.Height),
					Bandwidth:  strconv.Itoa(bandwidth),
				})
			case "h265", "hevc":
				// HEVC -> 2 variants: copy and x264
				log.Println("High efficiency stream detected. Copying and adding alternate h264 stream")
				variants = append(variants, VideoVariant{
					MapInput:   strconv.Itoa(inputIndex) + ":" + strconv.Itoa(masterVideoIndex),
					Codec:      "copy",
					Resolution: strconv.Itoa(videoStream.Width) + "x" + strconv.Itoa(videoStream.Height),
					Bandwidth:  strconv.Itoa(bandwidth),
				})
				// For the x264 variant, compute height setting
				h264Width, h264Height := computeNewRatio(videoStream)
				crf := 18
				variants = append(variants, VideoVariant{
					MapInput:         strconv.Itoa(inputIndex) + ":" + strconv.Itoa(masterVideoIndex),
					Codec:            "libx264",
					CRF:              &crf,
					ResolutionHeight: &h264Height,
					Resolution:       strconv.Itoa(h264Width) + "x" + strconv.Itoa(h264Height),
					Bandwidth:        strconv.Itoa(bandwidth),
				})
			default:
				// One variant: converter to x264, after computing height setting
				h264Width, h264Height := computeNewRatio(videoStream)
				crf := 18
				variants = append(variants, VideoVariant{
					MapInput:         strconv.Itoa(inputIndex) + ":" + strconv.Itoa(masterVideoIndex),
					Codec:            "libx264",
					CRF:              &crf,
					ResolutionHeight: &h264Height,
					Resolution:       strconv.Itoa(h264Width) + "x" + strconv.Itoa(h264Height),
					Bandwidth:        strconv.Itoa(bandwidth),
				})
			}

		}
	}
	return
}

func computeNewRatio(videoStream *probe.ProbeStream) (int, int) {
	h264Height := videoStream.Height // The height of the h264 stream to use
	if h264Height > 1080 {
		h264Height = 1080
		ratio := 1.777778 // Defaults to 16/9
		ratioStrings := strings.Split(videoStream.DisplayAspectRatio, ":")
		if len(ratioStrings) == 2 {
			a, err1 := strconv.ParseFloat(ratioStrings[0], 64)
			b, err2 := strconv.ParseFloat(ratioStrings[1], 64)
			if err1 == nil && err2 == nil {
				ratio = a / b
			} else {
				log.Println("WARNING: Cannot parse aspect ratio (" + videoStream.DisplayAspectRatio + "). Defaulting to 16/9")
			}
		} else {
			log.Println("WARNING: Unexpected aspect ratio format (" + videoStream.DisplayAspectRatio + "). Defaulting to 16/9")
		}
		return int(float64(h264Height) * ratio), h264Height
	} else {
		return videoStream.Width, videoStream.Height
	}
}