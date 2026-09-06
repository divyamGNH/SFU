"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useState, useRef, useEffect } from "react";
import type {
  ClientToServerMessage,
  PeerLeftMessage,
  ServerToClientMessage,
  SubscriberAnswerMessage,
} from "@/types/wsMessageTypes";
import { ErrorResponse, ViewRoomResponse } from "@/types/httpMessageTypes";

export default function WaitingPage() {
  const BASE_URL =
    process.env.NEXT_PUBLIC_BASE_URL ??
    process.env.NEXT_PUBLIC_API_URL ??
    "http://localhost:8080";

  const WS_BASE_URL = BASE_URL.replace(/^http/, "ws");

  const pcRef = useRef<RTCPeerConnection | null>(null);
  const subscriberPcRef = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const messageQueueRef = useRef<ClientToServerMessage[]>([]);
  const localVideoRef = useRef<HTMLVideoElement | null>(null);

  const [otherPeers, setOtherPeers] = useState<string[]>([]);
  const [isMicMuted, setIsMicMuted] = useState<boolean>(false);
  const [isVideoOff, setIsVideoOff] = useState<boolean>(false);
  const [peerMediaState, setPeerMediaState] = useState<
    Record<string, { audioMuted: boolean; videoMuted: boolean }>
  >({});

  const pendingIceCandidatesRef = useRef<RTCIceCandidateInit[]>([]);
  const pendingSubscriberIceCandidatesRef = useRef<RTCIceCandidateInit[]>([]);

  const pendingTracksRef = useRef<Record<string, RTCTrackEvent[]>>({});

  const remoteVideoRefs = useRef<Record<string, HTMLVideoElement | null>>({});

  const remoteStreamsRef = useRef<Record<string, MediaStream>>({});

  const hasInitializedRef = useRef<boolean>(false);

  const midToPublisherRef = useRef<Record<string, string | null>>({});

  const searchParams = useSearchParams();

  const roomId = searchParams.get("roomId");
  const userId = searchParams.get("userId");

  const router = useRouter();

  // Send ws messages Helper.
  function sendMessage(msg: ClientToServerMessage) {
    const ws = wsRef.current;

    // Check if the ws state, if not ready push to the messageQueue.
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      messageQueueRef.current.push(msg);
      return;
    }

    ws.send(JSON.stringify(msg));
  }

  // WS cleanup Helper
  const wsCleanup = () => {
    if (!wsRef.current) return;

    wsRef.current.onopen = null;
    wsRef.current.onerror = null;
    wsRef.current.onclose = null;
    wsRef.current.onmessage = null;
    wsRef.current.close();
    wsRef.current = null;
  };

  // PC cleanup Helper
  const pcCleanup = (pc: RTCPeerConnection | null) => {
    if (!pc) return;

    pc.onicecandidate = null;
    pc.ontrack = null;
    pc.onsignalingstatechange = null;
    pc.close();
  };

  // Add the pending ice candidates from the queue.
  async function flushPendingIceCandidates(
    pc: RTCPeerConnection | null,
    queue: RTCIceCandidateInit[],
  ) {
    if (!pc || !pc.remoteDescription) {
      return;
    }

    if (queue.length === 0) {
      return;
    }

    // Use the queue elements.
    for (const candidate of queue) {
      try {
        await pc.addIceCandidate(candidate);
      } catch (error) {
        console.error("Failed to add queued ICE candidate:", error);
      }
    }

    //Empty the queue.
    queue.length = 0;
  }

  // Decide wether ice candidate to be added directly or to the queue.
  async function queueOrAddIceCandidate(
    pc: RTCPeerConnection | null,
    candidate: RTCIceCandidateInit,
    queue: RTCIceCandidateInit[],
  ) {
    // If pc not set or remoteSDP not set push in the queue.
    if (!pc || !pc.remoteDescription) {
      queue.push(candidate);
      return;
    }

    try {
      await pc.addIceCandidate(candidate);
    } catch (error) {
      console.error("Failed to add ICE candidate:", error);
    }
  }

  //Get all the possible iceServers including STUN and TURN.
  async function getIceServers(): Promise<RTCIceServer[]> {
    const fallbackServers: RTCIceServer[] = [
      {
        urls: "stun:stun.l.google.com:19302",
      },
    ];

    // TODO : Add TURN servers but they need auth so need to add that first.
    return fallbackServers;
  }

  // Remove the peer that left from the room states.
  const removePeer = (leftUserId: string) => {
    setOtherPeers((prev) => prev.filter((peerId) => peerId !== leftUserId));
    setPeerMediaState((prev) => {
      if (!(leftUserId in prev)) {
        return prev;
      }

      const nextState = { ...prev };
      delete nextState[leftUserId];

      return nextState;
    });

    delete remoteVideoRefs.current[leftUserId];
    delete remoteStreamsRef.current[leftUserId];
  };

  const updatePeerMediaState = (
    peerId: string,
    nextState: Partial<{ audioMuted: boolean; videoMuted: boolean }>,
  ) => {
    setPeerMediaState((prev) => {
      const currentState = prev[peerId] ?? {
        audioMuted: false,
        videoMuted: false,
      };

      return {
        ...prev,
        [peerId]: {
          ...currentState,
          ...nextState,
        },
      };
    });
  };

  // To toggle either audio or video.
  const toggleLocalTrackEnabled = (kind: "audio" | "video"): boolean | null => {
    // Get the localstream.
    const stream = localStreamRef.current;
    if (!stream) return null;

    // Get the tracks.
    const tracks = kind === "audio" ? stream.getAudioTracks() : stream.getVideoTracks();
    if (tracks.length === 0) return null;

    // Toggle them.
    const nextEnabled = !tracks[0].enabled;
    tracks.forEach((track) => {
      track.enabled = nextEnabled;
    });

    if (kind === "audio") {
      setIsMicMuted(!nextEnabled);
      return !nextEnabled;
    }

    setIsVideoOff(!nextEnabled);
    return !nextEnabled;
  };

  function handleToggleMic() {
    const muted = toggleLocalTrackEnabled("audio");
    if (muted == null) return;

    sendMessage({
      type: "audio-toggle",
      muted
    });
  }

  function handleToggleVideo() {
    const muted = toggleLocalTrackEnabled("video");
    if (muted == null) return;

    sendMessage({
      type: "video-toggle",
      muted
    })
  }

  // handle the leave button or a unexpected leave of the user from the call.
  async function handleLeave() {
    if (!userId) return;
    if (!roomId) return;

    const res = await fetch(
      `http://localhost:8080/leaveroom/${roomId}/${userId}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    if (!res.ok) {
      const err: ErrorResponse = await res.json();
      console.log(
        `Error leaving room : Status code ${res.status} ${err.message}`,
      );
      return;
    }

    const msg: PeerLeftMessage = {
      type: "peer-left",
      roomId: roomId,
      userId: userId,
    }

    sendMessage(msg)

    // Stop all the local tracks majorly clients own audio and video.
    localStreamRef.current?.getTracks().forEach((track) => {
      track.stop();
    });

    pcCleanup(pcRef.current);
    pcCleanup(subscriberPcRef.current);
    wsCleanup();

    pcRef.current = null;
    subscriberPcRef.current = null;

    // TODO : Make sure all the states are cleaned up properly.
    hasInitializedRef.current = false;
    localStreamRef.current = null;
    localVideoRef.current = null;
    messageQueueRef.current = [];
    pendingIceCandidatesRef.current = [];
    pendingSubscriberIceCandidatesRef.current = [];
    remoteStreamsRef.current = {};
    remoteVideoRefs.current = {};

    router.push("/dashboard");
  }

  // Create short ids for UI purposes.
  const shortId = (id: string | null) => {
    if (!id) {
      console.log("A unknown userId probably null value has appeared.");

      return "unknown";
    }

    if (id.length <= 10) return id;

    return `${id.slice(0, 6)}...${id.slice(-4)}`;
  };

  //We did not put this in a try catch as this function is already called up in a try catch block itself and i need all the errors to be handleded by a single catch blocks.
  async function setupLocalStream() {
    console.log("Setting up the local stream");

    try {
      if (localStreamRef.current) {
        return;
      }

      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: true,
      });

      localStreamRef.current = stream;

      if (localVideoRef.current) {
        localVideoRef.current.srcObject = stream;
      }
    } catch (error) {
      console.log(error);

      throw new Error("setupLocalStream failed", {
        cause: error,
      });
    }
  }

  // Create a PC and set up all the PC listener for events like ontrack, onice etc.
  async function createPeerConnection() {
    console.log("called create peer connection");

    try {
      if (!localStreamRef.current) {
        throw new Error("Local stream not initialized");
      }

      if (!wsRef.current) {
        throw new Error("WebSocket not initialized");
      }

      const iceServers = await getIceServers();

      const pc = new RTCPeerConnection({
        iceServers,
      });

      // Declare the audio tranceiver.
      const audioTranceiver = pc.addTransceiver("audio", {
        direction: "sendonly",
      });

      // Declare the video tranceiver.
      const videoTranceiver = pc.addTransceiver("video", {
        direction: "sendonly",
      });

      // Send the audio and video through the tranceiver.
      await audioTranceiver.sender.replaceTrack(
        localStreamRef.current.getAudioTracks()[0],
      );

      await videoTranceiver.sender.replaceTrack(
        localStreamRef.current.getVideoTracks()[0],
      );

      pc.onicecandidate = (event) => {
        if (event.candidate) {
          sendMessage({
            type: "ice-candidate",
            iceCandidate: event.candidate.toJSON(),
          });
        }
      };

      // Publisher PC does not need tracks.

      pc.oniceconnectionstatechange = () => {
        console.log(`ICE connection state change: ${pc.iceConnectionState}`);
      };

      pc.onconnectionstatechange = () => {
        console.log(
          `Connection state: ${pc.connectionState} | ICE state: ${pc.iceConnectionState}`,
        );
      };

      pcRef.current = pc;

      return pc;
    } catch (error) {
      console.log(error);

      throw new Error("Error in creating the Peer Connection", {
        cause: error,
      });
    }
  }

  // Attach tracks to the correct peer stream and UI.
  function attachTrackToPeer(publisherId: string, track: MediaStreamTrack) {
    console.log(
      "ATTACHING",
      publisherId,
      track.kind,
      track.id,
      track.readyState
    );

    let stream = remoteStreamsRef.current[publisherId];

    if (!stream) {
      stream = new MediaStream();
      remoteStreamsRef.current[publisherId] = stream;
    }

    const alreadyExists = stream.getTracks().some((t) => t.id === track.id);

    if (!alreadyExists) {
      stream.addTrack(track);
    }

    const videoEl = remoteVideoRefs.current[publisherId];

    if (videoEl && videoEl.srcObject !== stream) {
      videoEl.srcObject = stream;

      videoEl.play()
        .then(() => console.log("PLAY OK"))
        .catch(err => console.log("PLAY ERR", err));

      console.log(
        "VIDEO ELEMENT",
        publisherId,
        videoEl?.muted,
        videoEl?.volume,
        videoEl?.paused,
        stream.getAudioTracks().length,
        stream.getVideoTracks().length
      );
    }
  }

  //Listen to all the web socket calls in real-time.
  async function setupWebSocketListeners(ws: WebSocket) {
    console.log("called setUpWebsocketListeners");

    ws.onmessage = async (event) => {
      const message: ServerToClientMessage = JSON.parse(event.data);

      switch (message.type) {
        case "answer":
          console.log("answer triggered");

          const pc = pcRef.current;

          if (!pc) return;

          await pc.setRemoteDescription(message.sdp);

          await flushPendingIceCandidates(pc, pendingIceCandidatesRef.current);

          break;

        case "ice-candidate":
          console.log("ice triggered");

          if (message.iceCandidate && pcRef.current) {
            await queueOrAddIceCandidate(
              pcRef.current,
              message.iceCandidate,
              pendingIceCandidatesRef.current,
            );
          }

          break;

        case "peer-joined":
          console.log("User joined room:", message.userId);

          if (message.userId !== userId) {
            setOtherPeers((prev) =>
              prev.includes(message.userId) ? prev : [...prev, message.userId],
            );
            updatePeerMediaState(message.userId, {
              audioMuted: false,
              videoMuted: false,
            });
          }

          break;

        case "peer-left":
          console.log("User left room:", message.userId);

          removePeer(message.userId);

          break;

        case "subscriber-offer":
          console.log(
            "Subscriber offer",
            message.sdp.type,
            message.sdp.sdp?.slice(0, 40),
            Date.now(),
          );

          // Get all the ice server
          const iceServers = await getIceServers();

          // create offer
          if (subscriberPcRef.current == null) {
            const subscriberPc = new RTCPeerConnection({
              iceServers,
            });

            subscriberPcRef.current = subscriberPc;

            subscriberPc.ontrack = (event) => {
              console.log(
                "TRACK RECEIVED",
                event.track.kind,
                event.track.id,
                event.transceiver.mid
              );

              const track = event.track;
              const mid = event.transceiver.mid;

              if (!mid) {
                console.log("MID missing");
                return;
              }

              const publisherId = midToPublisherRef.current[mid];

              // Queue track if publisher metadata not arrived yet
              if (!publisherId) {
                console.log("Queueing track for MID:", mid);

                if (!pendingTracksRef.current[mid]) {
                  pendingTracksRef.current[mid] = [];
                }

                pendingTracksRef.current[mid].push(event);

                return;
              }

              attachTrackToPeer(publisherId, track);

              console.log(`Attached ${track.kind} track for ${publisherId}`);
            };

            subscriberPc.onicecandidate = (event) => {
              if (event.candidate) {
                sendMessage({
                  type: "subscriber-ice-candidate",
                  iceCandidate: event.candidate.toJSON(),
                });
              }
            };

            subscriberPc.oniceconnectionstatechange = () => {
              console.log(
                `ICE connection state change for subscriber: ${subscriberPc.iceConnectionState}`,
              );
            };

            subscriberPc.onconnectionstatechange = () => {
              console.log(
                `Subscriber Connection state: ${subscriberPc.connectionState} | Subscriber ICE state: ${subscriberPc.iceConnectionState}`,
              );

              if (subscriberPc.connectionState === "connected") {
                setInterval(async () => {
                  const stats = await subscriberPc.getStats();

                  stats.forEach((report) => {
                    if (
                      report.type === "inbound-rtp" &&
                      report.kind === "audio"
                    ) {
                      // console.log(
                      //   "AUDIO RTP",
                      //   report.mid,
                      //   report.packetsReceived,
                      //   report.bytesReceived
                      // );
                    }
                  });
                }, 2000);
              }
            };
          }

          // Get the old subscriber pc.
          const subscriberPc = subscriberPcRef.current;

          // Set remoteDesc
          await subscriberPc.setRemoteDescription(message.sdp);

          // create subscriber answer
          const answer = await subscriberPc.createAnswer();

          await subscriberPc.setLocalDescription(answer);

          const subscriberAnswerMsg: SubscriberAnswerMessage = {
            type: "subscriber-answer",
            sdp: answer,
          };

          sendMessage(subscriberAnswerMsg);

          await flushPendingIceCandidates(
            subscriberPcRef.current,
            pendingSubscriberIceCandidatesRef.current,
          );

          break;

        case "subscriber-ice-candidate":
          console.log("Subscriber ice received");

          //create a queue
          //check if the remote desc is set if yes add them here else push to the queue
          if (message.iceCandidate && subscriberPcRef.current) {
            await queueOrAddIceCandidate(
              subscriberPcRef.current,
              message.iceCandidate,
              pendingSubscriberIceCandidatesRef.current,
            );
          }

          break;

        case "media-published":
          console.log("Received media metadata", message);

          const mid = message.mid;
          const publisher = message.publisher;

          midToPublisherRef.current[mid] = publisher;

          // Flush queued tracks for this MID
          const pendingEvents = pendingTracksRef.current[mid];

          if (pendingEvents) {
            for (const event of pendingEvents) {
              attachTrackToPeer(publisher, event.track);
            }

            delete pendingTracksRef.current[mid];
          }

          break;

        case "audio-toggle":
          updatePeerMediaState(message.userId, {
            audioMuted: message.muted,
          });

          break;

        case "video-toggle":
          updatePeerMediaState(message.userId, {
            videoMuted: message.muted,
          });

          break;

        default:
          console.log("Unknown websocket message:", message);
      }
    };
  }

  // Create all the neccesary things for the conference call.
  const settingRTCEnvironment = async () => {
    try {
      // Validate required parameters
      if (!roomId || !userId) {
        console.error("Missing roomId or userId from URL params");

        throw new Error(
          `Invalid URL params: roomId=${roomId}, userId=${userId}`,
        );
      }

      await setupLocalStream();

      // TODO : if we dont get a localStream should be really return the function ?
      if (!localStreamRef.current) return;

      if (
        wsRef.current &&
        (wsRef.current.readyState === WebSocket.CONNECTING ||
          wsRef.current.readyState === WebSocket.OPEN)
      ) {
        console.log("WebSocket already exists");
        return;
      }

      // Set up the WebSocket connection
      const wsUrl = `${WS_BASE_URL}/ws/${roomId}/${userId}`;

      console.log("Connecting to WebSocket:", wsUrl);

      const ws = new WebSocket(wsUrl);

      wsRef.current = ws;

      // Set up error handler BEFORE any other handlers
      ws.onerror = (event) => {
        console.error("WebSocket error:", event);

        // Check if connection was refused from the backend.
        if (event instanceof Event && event.type === "error") {
          console.error(
            `Failed to connect to WebSocket. Make sure backend is running on ${BASE_URL}`,
          );
        }
      };

      ws.onclose = () => {
        console.log("WebSocket disconnected");
      };

      // Set up open handler
      ws.onopen = async () => {
        console.log("ws.onopen triggered");

        // Clear the messaging queue.
        messageQueueRef.current.forEach((msg) => {
          ws.send(JSON.stringify(msg));
        });

        messageQueueRef.current = [];

        sendMessage({
          type: "populate-room",
          roomId: roomId,
          userId: userId,
        });

        console.log("WebSocket connected successfully");

        const pc = await createPeerConnection();

        const offer = await pc.createOffer();

        await pc.setLocalDescription(offer);

        sendMessage({
          type: "offer",
          sdp: offer,
        });
      };

      // Set up all the WS listeners
      await setupWebSocketListeners(ws);

      console.log("fetching other peers");

      const res = await fetch(`${BASE_URL}/viewroom/${roomId}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      });

      if (!res.ok) {
        const err: ErrorResponse = await res.json();

        throw new Error(`Failed fetching room peers: ${err.message}`);
      }

      console.log("fetched other peers");

      const response: ViewRoomResponse = await res.json();

      // TODO : Currently we are seperating the user manually later implement auth and remove the user in the backend itself and get the userId from the token
      const peers = Array.isArray(response.otherPeers)
        ? response.otherPeers.filter((peer) => peer.userId !== userId)
        : [];

      setOtherPeers(peers.map(p => p.userId));
      setPeerMediaState((prev) => {
        const nextState = { ...prev };

        for (const peer of peers) {
          nextState[peer.userId] = {
            audioMuted: peer.audioBool,
            videoMuted: peer.videoBool,
          };
        }

        return nextState;
      });
    } catch (error) {
      console.error("Error setting up RTC environment:", error);

      if (error instanceof Error) {
        console.error("Error details:", error.message);
      }
    }
  };

  useEffect(() => {
    if (hasInitializedRef.current) return;

    hasInitializedRef.current = true;

    const init = async () => {
      try {
        await settingRTCEnvironment();
      } catch (error) {
        console.log("Error setting up the RTC environment :", error);
      }
    };

    void init();

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    window.addEventListener("pagehide", handleLeave);

    return () => {
      window.removeEventListener("pagehide", handleLeave);
    };

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    return () => {
      localStreamRef.current?.getTracks().forEach((track) => {
        track.stop();
      });

      pcCleanup(pcRef.current);
      pcCleanup(subscriberPcRef.current);
      wsCleanup();

      pcRef.current = null;
      subscriberPcRef.current = null;

      if (localVideoRef.current) {
        localVideoRef.current.srcObject = null;
      }

      localStreamRef.current = null;
      setPeerMediaState({});
    };
  }, []);

  return (
    <div className="relative min-h-screen overflow-hidden bg-slate-950 p-4 text-slate-100 sm:p-6 lg:p-8">
      <div className="pointer-events-none absolute -left-18 top-20 h-56 w-56 rounded-full border border-cyan-400/20" />

      <div className="pointer-events-none absolute -right-20 bottom-12 h-60 w-60 rounded-full border border-indigo-400/20" />

      <div className="relative mx-auto max-w-7xl space-y-6">
        <div className="rounded-2xl border border-slate-700 bg-slate-900/90 px-4 py-4 shadow-xl shadow-black/30 sm:px-6">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                Room Session
              </p>

              <p className="mt-1 font-mono text-sm text-cyan-200 sm:text-base">
                {roomId || "unknown"}
              </p>
            </div>

            <div className="flex items-center gap-2">
              <span className="rounded-full border border-emerald-500/40 bg-emerald-500/10 px-3 py-1 text-xs font-medium text-emerald-200">
                Live
              </span>

              <span className="rounded-full border border-slate-600 bg-slate-800 px-3 py-1 text-xs text-slate-200">
                Peers: {otherPeers?.length ?? 1}
              </span>
            </div>
          </div>

          <div className="mt-4 flex flex-wrap items-center gap-3 border-t border-slate-700/70 pt-4">
            <div className="rounded-md border border-slate-600 bg-slate-800 px-3 py-1.5 text-xs text-slate-300 sm:text-sm">
              You{" "}
              <span className="font-mono text-slate-100">
                {shortId(userId)}
              </span>
            </div>

            <button
              onClick={() => {
                void handleLeave();
              }}
              className="ml-auto rounded-md border border-rose-400/50 bg-rose-400/15 px-3 py-1.5 text-xs font-semibold text-rose-200 transition hover:scale-[1.02] hover:bg-rose-400/25"
            >
              Leave Room
            </button>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-3">
          <div className="group relative overflow-hidden rounded-2xl border border-cyan-400/35 bg-slate-900 shadow-lg shadow-cyan-950/20 transition duration-300 hover:-translate-y-0.5 hover:border-cyan-300/60">
            <video
              ref={localVideoRef}
              autoPlay
              muted
              playsInline
              className={`h-64 w-full object-cover transition-opacity duration-300 sm:h-72 ${isVideoOff ? "opacity-0" : "opacity-100"}`}
            />

            {isVideoOff ? (
              <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-slate-950/95 px-4 text-center">
                <div className="flex h-16 w-16 items-center justify-center rounded-full border border-slate-700 bg-slate-900 text-slate-300">
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="h-8 w-8"
                  >
                    <path d="M15 10.5V8a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-2.5l5 3.5V7l-5 3.5Z" />
                    <path d="M4 4l16 16" />
                  </svg>
                </div>

                <p className="text-sm font-medium text-slate-200">Camera off</p>
              </div>
            ) : null}

            <div className="absolute inset-x-0 bottom-0 h-20 bg-linear-to-t from-black/70 to-transparent" />

            <div className="absolute bottom-3 right-3 flex items-center gap-2 rounded-full border border-slate-700/80 bg-black/60 px-2.5 py-1.5 backdrop-blur-sm">
              <span
                className={`flex h-6 w-6 items-center justify-center rounded-full border ${isMicMuted ? "border-rose-400/70 bg-rose-500/20 text-rose-200" : "border-slate-500/60 bg-slate-800 text-slate-100"}`}
                aria-label={isMicMuted ? "Microphone muted" : "Microphone on"}
              >
                <svg
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="h-3.5 w-3.5"
                >
                  <path d="M12 15a3 3 0 0 0 3-3V7a3 3 0 1 0-6 0v5a3 3 0 0 0 3 3Z" />
                  <path d="M19 11a7 7 0 0 1-14 0" />
                  <path d="M12 18v3" />
                  <path d="M9 21h6" />
                  {isMicMuted ? <path d="M4 4l16 16" /> : null}
                </svg>
              </span>

              <span className="text-[11px] font-medium text-slate-200">
                {isMicMuted ? "Muted" : "Live"}
              </span>
            </div>

            <div className="absolute bottom-3 left-3 rounded-md border border-cyan-300/40 bg-black/60 px-2.5 py-1 text-xs font-medium text-cyan-100">
              You | {shortId(userId)}
            </div>
          </div>

          {otherPeers.map((peerId) => {
            const peerState = peerMediaState[peerId] ?? {
              audioMuted: false,
              videoMuted: false,
            };

            return (
              <div
                key={peerId}
                className="group relative overflow-hidden rounded-2xl border border-slate-700 bg-slate-900 shadow-lg shadow-black/30 transition duration-300 hover:-translate-y-0.5 hover:border-indigo-300/55"
              >
                <video
                  ref={(el) => {
                    remoteVideoRefs.current[peerId] = el;

                    if (el && remoteStreamsRef.current[peerId]) {
                      el.srcObject = remoteStreamsRef.current[peerId];
                    }
                  }}
                  autoPlay
                  playsInline
                  muted={false}
                  className={`h-64 w-full object-cover transition-opacity duration-300 sm:h-72 ${peerState.videoMuted ? "opacity-0" : "opacity-100"}`}
                />

                {peerState.videoMuted ? (
                  <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-slate-950/95 px-4 text-center">
                    <div className="flex h-16 w-16 items-center justify-center rounded-full border border-slate-700 bg-slate-900 text-slate-300">
                      <svg
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="1.8"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        className="h-8 w-8"
                      >
                        <path d="M15 10.5V8a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-2.5l5 3.5V7l-5 3.5Z" />
                        <path d="M4 4l16 16" />
                      </svg>
                    </div>

                    <p className="text-sm font-medium text-slate-200">Camera off</p>
                  </div>
                ) : null}

                <div className="absolute inset-x-0 bottom-0 h-20 bg-linear-to-t from-black/70 to-transparent" />

                <div className="absolute bottom-3 right-3 flex items-center gap-2 rounded-full border border-slate-700/80 bg-black/60 px-2.5 py-1.5 backdrop-blur-sm">
                  <span
                    className={`flex h-6 w-6 items-center justify-center rounded-full border ${peerState.audioMuted ? "border-rose-400/70 bg-rose-500/20 text-rose-200" : "border-slate-500/60 bg-slate-800 text-slate-100"}`}
                    aria-label={peerState.audioMuted ? "Microphone muted" : "Microphone on"}
                  >
                    <svg
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.8"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="h-3.5 w-3.5"
                    >
                      <path d="M12 15a3 3 0 0 0 3-3V7a3 3 0 1 0-6 0v5a3 3 0 0 0 3 3Z" />
                      <path d="M19 11a7 7 0 0 1-14 0" />
                      <path d="M12 18v3" />
                      <path d="M9 21h6" />
                      {peerState.audioMuted ? <path d="M4 4l16 16" /> : null}
                    </svg>
                  </span>

                  <span className="text-[11px] font-medium text-slate-200">
                    {peerState.audioMuted ? "Muted" : "Live"}
                  </span>
                </div>

                <div className="absolute bottom-3 left-3 rounded-md border border-indigo-300/35 bg-black/60 px-2.5 py-1 text-xs font-medium text-slate-100">
                  Peer | {shortId(peerId)}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div className="pointer-events-none fixed inset-x-0 bottom-6 z-20 flex justify-center px-4">
        <div className="pointer-events-auto flex items-center gap-3 rounded-full border border-slate-700/80 bg-slate-900/90 px-4 py-3 shadow-2xl shadow-black/40 backdrop-blur-md">
          <button
            type="button"
            onClick={handleToggleMic}
            aria-pressed={isMicMuted}
            aria-label={isMicMuted ? "Unmute microphone" : "Mute microphone"}
            className={`flex h-12 w-12 items-center justify-center rounded-full border transition duration-200 ${isMicMuted
              ? "border-rose-400/60 bg-rose-500/15 text-rose-200 hover:bg-rose-500/25"
              : "border-slate-600 bg-slate-800 text-slate-100 hover:bg-slate-700"
              }`}
          >
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-5 w-5"
            >
              <path d="M12 15a3 3 0 0 0 3-3V7a3 3 0 1 0-6 0v5a3 3 0 0 0 3 3Z" />
              <path d="M19 11a7 7 0 0 1-14 0" />
              <path d="M12 18v3" />
              <path d="M9 21h6" />
              {isMicMuted ? <path d="M4 4l16 16" /> : null}
            </svg>
          </button>

          <div className="h-8 w-px bg-slate-700" />

          <button
            type="button"
            onClick={handleToggleVideo}
            aria-pressed={isVideoOff}
            aria-label={isVideoOff ? "Turn camera on" : "Turn camera off"}
            className={`flex h-12 w-12 items-center justify-center rounded-full border transition duration-200 ${isVideoOff
              ? "border-rose-400/60 bg-rose-500/15 text-rose-200 hover:bg-rose-500/25"
              : "border-slate-600 bg-slate-800 text-slate-100 hover:bg-slate-700"
              }`}
          >
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-5 w-5"
            >
              <path d="M15 10.5V8a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-2.5l5 3.5V7l-5 3.5Z" />
              {isVideoOff ? <path d="M4 4l16 16" /> : null}
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
