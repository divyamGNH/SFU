//TODO : Add tranceivers instead of AddTrack.
//TODO : Handle the received media correctly basically fix onTrack function.
//TODO : Backend does not have auth add auth here.

"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useState, useRef, useEffect } from "react";

// type RoomClient = {
//   clientId: string;
// };

// type TurnCredentialsResponse = {
//   username: string;
//   credential: string;
//   urls: string[];
//   ttl: number;
// };

export default function WaitingPage() {
  const BASE_URL =
    process.env.NEXT_PUBLIC_BASE_URL ??
    process.env.NEXT_PUBLIC_API_URL ??
    "http://localhost:8080";

  const WS_BASE_URL = BASE_URL.replace(/^http/, "ws");

  const pcRef = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const messageQueueRef = useRef<unknown[]>([]);
  const localVideoRef = useRef<HTMLVideoElement | null>(null);

  const [otherPeers, setOtherPeers] = useState<string[]>([]);

  const pendingIceCandidatesRef = useRef<RTCIceCandidateInit[]>([]);
  const remoteVideoRefs = useRef<Record<string, HTMLVideoElement | null>>({});
  const hasInitializedRef = useRef<boolean>(false);

  const searchParams = useSearchParams();

  const roomId = searchParams.get("roomId");
  const clientId = searchParams.get("clientId");

  const router = useRouter();

  // Send ws messages.
  function sendMessage(msg: unknown) {
    const ws = wsRef.current;

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
  const pcCleanup = () => {
    const pc = pcRef.current;

    if (!pc) return;

    pc.onicecandidate = null;
    pc.ontrack = null;
    pc.onsignalingstatechange = null;
    pc.close();

    pcRef.current = null;
  };

  // Add the pending ice candidates from the queue.
  async function flushPendingIceCandidates() {
    const pc = pcRef.current;

    if (!pc || !pc.remoteDescription) {
      return;
    }

    //Get the queue
    const queue = pendingIceCandidatesRef.current || [];

    if (queue.length === 0) {
      return;
    }

    for (const candidate of queue) {
      try {
        await pc.addIceCandidate(candidate);
      } catch (error) {
        console.error("Failed to add queued ICE candidate:", error);
      }
    }

    //Empty the queue.
    pendingIceCandidatesRef.current = [];
  }

  // Decide wether ice candidate to be added directly or to the queue.
  async function queueOrAddIceCandidate(candidate: RTCIceCandidateInit) {
    const pc = pcRef.current;

    if (!pc || !pc.remoteDescription) {
      const queued = pendingIceCandidatesRef.current || [];

      queued.push(candidate);

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

    //TODO : Use the implemented TURN server.
    // if (!roomId || !clientId) {
    //   return fallbackServers;
    // }

    // try {
    //   const res = await fetch(
    //     `${BASE_URL}/room/${roomId}/${clientId}/turn-credentials`,
    //   );

    //   if (!res.ok) {
    //     const txt = await res.text();

    //     console.warn(
    //       `TURN credentials unavailable (${res.status}). Falling back to STUN only. ${txt}`,
    //     );

    //     return fallbackServers;
    //   }

    //   const turn = (await res.json()) as TurnCredentialsResponse;

    //   if (!turn.urls?.length || !turn.username || !turn.credential) {
    //     return fallbackServers;
    //   }

    //   return [
    //     {
    //       urls: "stun:stun.l.google.com:19302",
    //     },
    //     {
    //       urls: turn.urls,
    //       username: turn.username,
    //       credential: turn.credential,
    //     },
    //   ];
    // } catch (error) {
    //   console.warn("Failed fetching TURN credentials, using STUN only", error);

    //   return fallbackServers;
    // }

    return fallbackServers;
  }

  // Remove the peer that left from the room states.
  const removePeer = (leftClientId: string) => {
    setOtherPeers((prev) => prev.filter((peerId) => peerId !== leftClientId));

    delete remoteVideoRefs.current[leftClientId];
  };

  // handle the leave button or a unexpected leave of the user from the call.
  async function handleLeave() {
    if (!clientId) return;

    const res = await fetch(
      `http://localhost:8080/leaveroom/${roomId}/${clientId}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    if (res.status != 200) {
      console.log("Error leaving room");
      return;
    }

    pcCleanup();
    wsCleanup();

    localStreamRef.current = null;
    localVideoRef.current = null;
    messageQueueRef.current = [];
    pendingIceCandidatesRef.current = [];

    router.push("/dashboard");
  }

  // Create short ids for UI purposes.
  const shortId = (id: string | null) => {
    if (!id) return "unknown";

    if (id.length <= 10) return id;

    return `${id.slice(0, 6)}...${id.slice(-4)}`;
  };

  // async function leaveCall() {
  //   const closeRes = await fetch(
  //     `${BASE_URL}/leaveroom/${roomId}/${clientId}`,
  //     {
  //       method: "DELETE",
  //     },
  //   );

  //   return closeRes;
  // }

  // async function handleLeaveClick() {
  //   console.log("Leaving the call");

  //   try {
  //     await leaveCall();

  //     wsRef.current?.close();

  //     router.push("/dashboard");
  //   } catch (error) {
  //     console.error("Failed to leave room:", error);
  //   }
  // }

  //We did not put this in a try catch as this function is already called up in a try catch block itself and i need all the errors to be handleded by a single catch blocks.
  async function setupLocalStream() {
    console.log("Setting up the local stream");
    try {
      const devices = await navigator.mediaDevices.enumerateDevices();

      const hasCamera = devices.some((d) => d.kind === "videoinput");

      console.log(hasCamera);

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

      throw new Error("setupLocalStream failed", { cause: error });
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

      localStreamRef.current.getTracks().forEach((track) => {
        pc.addTrack(track, localStreamRef.current!);
      });

      pc.onicecandidate = (event) => {
        if (event.candidate) {
          sendMessage({
            type: "ice-candidate",
            iceCandidate: event.candidate.toJSON(),
          });
        }
      };

      pc.ontrack = (event) => {
        const stream = event.streams[0];

        //TODO : Some-how receive the userId of the stream from the backend and store them in remoteVideoRefs
      };

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

  //Listen to all the web socket calls in real-time.
  async function setupWebSocketListeners(ws: WebSocket) {
    console.log("called setUpWebsocketListeners");
    ws.onmessage = async (event) => {
      const message = JSON.parse(event.data);

      switch (message.type) {
        case "answer":
          console.log("answer triggered");

          const pc = pcRef.current;

          if (!pc) return;

          await pc.setRemoteDescription(message.sdp);

          await flushPendingIceCandidates();

          break;

        case "ice-candidate":
          console.log("ice triggered");

          if (message.iceCandidate && pcRef.current) {
            await queueOrAddIceCandidate(message.iceCandidate);
          }

          break;

        case "peer-joined":
          console.log("User joined room:", message.userId);

          if (message.userId !== clientId) {
            setOtherPeers((prev) =>
              prev.includes(message.userId) ? prev : [...prev, message.userId],
            );
          }

          break;

        case "peer-left":
          console.log("User left room:", message.clientId);
          removePeer(message.userId);
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
      if (!roomId || !clientId) {
        console.error("Missing roomId or clientId from URL params");

        throw new Error(
          `Invalid URL params: roomId=${roomId}, clientId=${clientId}`,
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
      const wsUrl = `${WS_BASE_URL}/ws/${roomId}/${clientId}`;

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

      ws.onclose = async () => {
        const closeRes = await handleLeave();
        // await closeRes.json();

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
          type : "populate-room",
          roomId : roomId,
          clientId : clientId,
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

      //TODO : Implement this viewroom in the backend.
      console.log("fetching other peers");

      const res = await fetch(`${BASE_URL}/viewroom/${roomId}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      });

      if (!res.ok) {
        const txt = await res.text();

        throw new Error(`Failed fetching room peers: ${txt}`);
      }

      const response = await res.json();

      console.log("fetched other peers");

      // TODO : Currently we are seperating the client manually later implement auth and remove the client in the backend itself and get the clientId from the token 
      const peers = Array.isArray(response.otherPeers)
        ? response.otherPeers.filter((peerId: string) => peerId !== clientId)
        : [];

      setOtherPeers(peers);
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
      wsCleanup();
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
              You:{" "}
              <span className="font-mono text-slate-100">
                {shortId(clientId)}
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
              className="h-64 w-full object-cover sm:h-72"
            />

            <div className="absolute inset-x-0 bottom-0 h-20 bg-linear-to-t from-black/70 to-transparent" />

            <div className="absolute bottom-3 left-3 rounded-md border border-cyan-300/40 bg-black/60 px-2.5 py-1 text-xs font-medium text-cyan-100">
              You | {shortId(clientId)}
            </div>
          </div>

          {otherPeers.map((peerId) => (
            <div
              key={peerId}
              className="group relative overflow-hidden rounded-2xl border border-slate-700 bg-slate-900 shadow-lg shadow-black/30 transition duration-300 hover:-translate-y-0.5 hover:border-indigo-300/55"
            >
              <video
                ref={(el) => {
                  remoteVideoRefs.current[peerId] = el;
                }}
                autoPlay
                playsInline
                className="h-64 w-full object-cover sm:h-72"
              />

              <div className="absolute inset-x-0 bottom-0 h-20 bg-linear-to-t from-black/70 to-transparent" />

              <div className="absolute bottom-3 left-3 rounded-md border border-indigo-300/35 bg-black/60 px-2.5 py-1 text-xs font-medium text-slate-100">
                Peer | {shortId(peerId)}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
