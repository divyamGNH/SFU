package sfu

import (
	"log"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

// the actual WriteRTCP function needs a PC so basically a call looks like publisher.PC.WriteRTCP but to prevent coupling we inject that function in the sfu.
type RTCPWriter interface {
	WriteRTCP(packets []rtcp.Packet) error
}

type Receiver struct {
	trackRemote *webrtc.TrackRemote
	localTrack  *webrtc.TrackLocalStaticRTP
	rtcpWriter  RTCPWriter
	done        chan struct{}
}

func NewReceiver(trackRemote *webrtc.TrackRemote, localTrack *webrtc.TrackLocalStaticRTP, writer RTCPWriter) *Receiver {
	r := &Receiver{
		trackRemote: trackRemote,
		localTrack:  localTrack,
		rtcpWriter:  writer,
		done:        make(chan struct{}),
	}

	// Start the receiver loop.
	r.start()
	return r
}

// Start the loop as a seperate go routine.
// Here basically a publisher is reading its local media like camera audio etc and sending it to the sfu.
// trackRemote is basically sent by the frontend which we store as localMedia to be sent to the other peers.
func (r *Receiver) start() {
	go func() {
		for {
			select {
			case <-r.done:
				return
			default:
				// Read the packets from the remote track.
				packet, _, err := r.trackRemote.ReadRTP()
				if err != nil {
					log.Println("[SFU/Receiver] Error reading RTP packet:", err)
					return
				}

				// Write these packets to the localTrack.
				err = r.localTrack.WriteRTP(packet)
				if err != nil {
					log.Println("[SFU/Receiver] Error forwarding RTP packet:", err)
					return
				}
			}
		}
	}()
}

// SendPLI (Picture Loss Indication)
// Sending the PLI request upstream.
// trackRemote is the track we receive from the frontend basically SFU says that hey client this is the track u sent me another client needs a I-frame for this.
func (r *Receiver) SendPLI() {
	err := r.rtcpWriter.WriteRTCP([]rtcp.Packet{
		&rtcp.PictureLossIndication{
			MediaSSRC: uint32(r.trackRemote.SSRC()),
		},
	})
	if err != nil {
		log.Println("[SFU/Receiver] Error sending PLI upstream:", err)
		return
	}
	log.Println("[SFU/Receiver] Successfully sent PLI upstream")
}

// Closing the done channel.
// DoneCh was actually sleeping as we never really pushed anything into it but when we push close signal it wakes up and the select reads it and returns.
func (r *Receiver) Close() {
	close(r.done)
}
