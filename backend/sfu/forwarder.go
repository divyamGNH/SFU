package sfu

import (
	"backend/logger"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Forwarder struct {
	source *Receiver
	sender *webrtc.RTPSender
}

func NewForwarder(source *Receiver, sender *webrtc.RTPSender) *Forwarder {
	return &Forwarder{
		source: source,
		sender: sender,
	}
}

// Drain RTCP is basically a feedback from the client we just sent a stream to.
func (f *Forwarder) DrainRTCP() {
	// Create a buffer
	rtcpBuf := make([]byte, 1500)

	for {
		n, _, err := f.sender.Read(rtcpBuf)
		if err != nil {
			logger.Error("[SFU/Forwarder] error reading RTCP sender closed:", err)
			return
		}

		packets, err := rtcp.Unmarshal(rtcpBuf[:n])
		if err != nil {
			logger.Error("[SFU/Forwarder] error unmarshalling rtcp:", err)
			return
		}

		for _, packet := range packets {
			switch packet.(type) {
			case *rtcp.PictureLossIndication:
				logger.Info("[SFU] PLI received.")
				f.source.SendPLI()

			case *rtcp.FullIntraRequest:
				logger.Info("[SFU] FIR received")
				f.source.SendPLI()

			case *rtcp.TransportLayerNack:
				logger.Info("[SFU] Transport layer NACK received")

			case *rtcp.ReceiverReport:
				// logger.Info("[SFU] Receviver Report received")
			}
		}
	}
}
