package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/asticode/go-astiav"
	joy5 "github.com/nareix/joy5/codec/h264"
	"github.com/pion/webrtc/v4/pkg/media/h264reader"
)

var (
	decodeCodecContext *astiav.CodecContext
	decodePacket       *astiav.Packet
	decodeFrame        *astiav.Frame

	softwareScaleContext *astiav.SoftwareScaleContext
	scaledFrame          *astiav.Frame

	encodeCodecContext *astiav.CodecContext
	encodePacket       *astiav.Packet
	outFile            *os.File

	h264Reader             *h264reader.H264Reader
	printedVideoDimensions bool
	SPSAndPPSCache         []byte

	pts int64
	err error
)

const (
	outputWidth  = 1280
	outputHeight = 720
)

func main() {
	initVideoDecoding()
	defer freeVideoCoding()

	file, err := os.Open("in.h264")
	if err != nil {
		panic(err)
	}

	outFile, err = os.OpenFile("out.h264", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		panic(err)
	}

	h264Reader, err = h264reader.NewReader(file)
	if err != nil {
		panic(err)
	}

	for {
		// Read H264 from file
		if err = decodePacket.FromData(getNAL()); err != nil {
			panic(err)
		}

		// Send the H264 into Decoder
		if err = decodeCodecContext.SendPacket(decodePacket); err != nil {
			panic(err)
		}

		for {
			// Read Decoded Frame
			if err = decodeCodecContext.ReceiveFrame(decodeFrame); err != nil {
				if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
					break
				}
				panic(err)
			}

			// Init the Encoding+Scaling. Can't be started until we know info on input video
			initVideoEncoding()

			// Scale the video
			if err := softwareScaleContext.ScaleFrame(decodeFrame, scaledFrame); err != nil {
				panic(err)
			}

			// We don't care about the PTS, but encoder complains if unset
			pts++
			scaledFrame.SetPts(pts)

			// Encode the frame
			if err := encodeCodecContext.SendFrame(scaledFrame); err != nil {
				panic(err)
			}

			for {
				// Read encoded packets and write to file
				encodePacket = astiav.AllocPacket()
				if err := encodeCodecContext.ReceivePacket(encodePacket); err != nil {
					if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
						break
					}
					panic(err)
				}

				if _, err := outFile.Write(encodePacket.Data()); err != nil {
					panic(err)
				}
			}
		}
	}

}

func getNAL() []byte {
	for {
		nal, err := h264Reader.NextNAL()
		if err != nil {
			panic(err)
		}

		if nal.UnitType == joy5.NALU_SPS && !printedVideoDimensions {
			info, err := joy5.ParseSPS(nal.Data)
			if err != nil {
				panic(err)
			}

			fmt.Printf("Video Dimensions %dx%d\n", info.Width, info.Height)
			printedVideoDimensions = true
		}

		nal.Data = append([]byte{0x0, 0x0, 0x1}, nal.Data...)
		if nal.UnitType == joy5.NALU_SPS || nal.UnitType == joy5.NALU_PPS {
			SPSAndPPSCache = append(SPSAndPPSCache, nal.Data...)
			continue
		}

		if len(SPSAndPPSCache) != 0 {
			nal.Data = append(SPSAndPPSCache, nal.Data...)
			SPSAndPPSCache = []byte{}
		}

		return nal.Data
	}
}

func initVideoDecoding() {
	h264Decoder := astiav.FindDecoderByName("h264")
	if h264Decoder == nil {
		panic("No H264 Decoder Found")
	}

	if decodeCodecContext = astiav.AllocCodecContext(h264Decoder); decodeCodecContext == nil {
		panic("Failed to AllocCodecContext Decoder")
	}

	if err := decodeCodecContext.Open(h264Decoder, nil); err != nil {
		panic(err)
	}

	decodePacket = astiav.AllocPacket()
	decodeFrame = astiav.AllocFrame()
}

func initVideoEncoding() {
	if encodeCodecContext != nil {
		return
	}

	h264Encoder := astiav.FindEncoderByName("libx264")
	if h264Encoder == nil {
		panic("No H264 Encoder Found")
	}

	if encodeCodecContext = astiav.AllocCodecContext(h264Encoder); encodeCodecContext == nil {
		panic("Failed to AllocCodecContext Decoder")
	}

	encodeCodecContext.SetPixelFormat(decodeCodecContext.PixelFormat())
	encodeCodecContext.SetSampleAspectRatio(decodeCodecContext.SampleAspectRatio())
	encodeCodecContext.SetTimeBase(astiav.NewRational(1, 30))
	encodeCodecContext.SetWidth(outputWidth)
	encodeCodecContext.SetHeight(outputHeight)

	if err := encodeCodecContext.Open(h264Encoder, nil); err != nil {
		panic(err)
	}

	softwareScaleContext, err = astiav.CreateSoftwareScaleContext(
		decodeCodecContext.Width(),
		decodeCodecContext.Height(),
		decodeCodecContext.PixelFormat(),
		outputWidth,
		outputHeight,
		decodeCodecContext.PixelFormat(),
		astiav.NewSoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagBilinear),
	)
	if err != nil {
		panic(err)
	}

	scaledFrame = astiav.AllocFrame()
}

func freeVideoCoding() {
	decodeCodecContext.Free()
	decodePacket.Free()
	decodeFrame.Free()

	softwareScaleContext.Free()
	scaledFrame.Free()

	encodeCodecContext.Free()
	encodePacket.Free()
}
