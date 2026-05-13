"use client";

import { useEffect, useRef } from "react";

export default function Home() {
  const localVideoRef = useRef<HTMLVideoElement | null>(null);

  const pcRef = useRef<RTCPeerConnection | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const hasStarted = useRef(false);

  useEffect(() => {
    if (hasStarted.current) return;

    hasStarted.current = true;

    async function start() {
      try {
        console.log("[Frontend] Starting frontend");

        // Create websocket connection
        const ws = new WebSocket("ws://localhost:8080/ws");

        wsRef.current = ws;

        // Create peer connection
        const pc = new RTCPeerConnection({
          iceServers: [
            {
              urls: "stun:stun.l.google.com:19302",
            },
          ],
        });

        pcRef.current = pc;

        console.log("[Frontend] RTCPeerConnection created");

        // Send ICE candidates to backend
        pc.onicecandidate = (event) => {
          if (!event.candidate) {
            return;
          }

          if (ws.readyState !== WebSocket.OPEN) {
            console.log("[Frontend] Websocket not open");
            return;
          }

          ws.send(
            JSON.stringify({
              type: "ice-candidate",
              candidate: event.candidate,
            }),
          );
        };

        pc.onconnectionstatechange = () => {
          console.log(
            "[Connection State]",
            pc.connectionState,
          );
        };

        pc.oniceconnectionstatechange = () => {
          console.log(
            "[ICE State]",
            pc.iceConnectionState,
          );
        };

        pc.onicegatheringstatechange = () => {
          console.log(
            "[ICE Gathering]",
            pc.iceGatheringState,
          );
        };

        pc.onsignalingstatechange = () => {
          console.log(
            "[Signaling State]",
            pc.signalingState,
          );
        };

        pc.ontrack = (event) => {
          console.log("[Frontend] Remote track received");

          console.log(event.streams);
        };

        ws.onopen = async () => {
          console.log("[Frontend] Websocket connected");

          try {
            // Get local media
            const stream = await navigator.mediaDevices.getUserMedia({
              video: true,
              audio: true,
            });

            console.log("[Frontend] Local media stream received");

            // Show local video
            if (localVideoRef.current) {
              localVideoRef.current.srcObject = stream;
            }

            // Add tracks to peer connection
            stream.getTracks().forEach((track) => {
              pc.addTrack(track, stream);
            });

            console.log("[Frontend] Local tracks added");

            // Create offer
            const offer = await pc.createOffer();

            // Set local description
            await pc.setLocalDescription(offer);

            console.log("[Frontend] Local description set");

            // Send offer to backend
            ws.send(
              JSON.stringify({
                type: "offer",
                sdp: offer,
              }),
            );

            console.log("[Frontend] Offer sent");
          } catch (error) {
            console.log(
              "[Frontend] Error inside websocket onopen:",
              error,
            );
          }
        };

        ws.onmessage = async (event) => {
          const message = JSON.parse(event.data);

          if (message.type === "answer" && message.sdp) {
            console.log("[Frontend] Answer received");

            await pc.setRemoteDescription(message.sdp);

            console.log("[Frontend] Remote description set");

            return;
          }

          if (message.type === "ice-candidate") {
            try {
              await pc.addIceCandidate(message.ICECandidate);
            } catch (error) {
              console.log(
                "[Frontend] Error adding ICE candidate:",
                error,
              );
            }

            return;
          }

          console.log("[Frontend] Unknown websocket message");
        };

        ws.onerror = (error) => {
          console.log("[Frontend] Websocket error:", error);
        };

        ws.onclose = () => {
          console.log("[Frontend] Websocket closed");
        };
      } catch (error) {
        console.log("[Frontend] Error occurred:", error);
      }
    }

    start();

    return () => {
      console.log("[Frontend] Cleaning up");

      pcRef.current?.close();
      wsRef.current?.close();
    };
  }, []);

  return (
    <div className="min-h-screen bg-black flex flex-col items-center justify-center gap-10">
      <h1 className="text-white text-4xl font-bold">
        Minimal SFU Frontend Test
      </h1>

      <video
        ref={localVideoRef}
        autoPlay
        muted
        playsInline
        className="w-[700px] rounded-xl border border-white"
      />
    </div>
  );
}