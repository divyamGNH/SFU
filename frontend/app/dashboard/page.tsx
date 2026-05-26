"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export default function Dashboard() {
  const BASE_URL =
    process.env.NEXT_PUBLIC_BASE_URL ??
    process.env.NEXT_PUBLIC_API_URL ??
    "http://localhost:8080";

  const [roomID, setRoomID] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [connectingAction, setConnectingAction] = useState<
    "create" | "join" | null
  >(null);

  const router = useRouter();

  const handleCreateRoom = async () => {
    if (connecting) return;

    setConnecting(true);
    setConnectingAction("create");

    try {
      const res = await fetch(`${BASE_URL}/createroom`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      });

      const { roomId, userId } = await res.json();

      console.log(roomId);
      console.log(userId);

      router.push(`/waitingRoom?roomId=${roomId}&userId=${userId}`);
    } catch (error) {
      console.log("Error creating the room : ", error);
    } finally {
      setConnecting(false);
      setConnectingAction(null);
    }
  };

  const handleJoinRoom = async () => {
    // Join room using the room ID entered in the input field
    if (connecting) return;

    const targetRoomId = roomID.trim();

    if (!targetRoomId) {
      console.log("Room ID is required");
      return;
    }

    try {
      setConnecting(true);
      setConnectingAction("join");

      const res = await fetch(`${BASE_URL}/joinroom/${targetRoomId}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      });

      if (!res.ok) {
        const errText = await res.text();

        throw new Error(errText || `Join failed with status ${res.status}`);
      }

      const data = await res.json();

      console.log(data);

      const { roomId, userId } = data;

      console.log(roomId);
      console.log(userId);

      router.push(`/waitingRoom?roomId=${roomId}&userId=${userId}`);
    } catch (error) {
      console.log(`Error joining the room id=${targetRoomId}: `, error);
    } finally {
      setConnecting(false);
      setConnectingAction(null);
    }
  };

  return (
    <div className="relative min-h-screen overflow-hidden bg-slate-950 px-6 py-8 text-slate-100 sm:px-10">
      <div className="pointer-events-none absolute -left-12 top-20 h-44 w-44 rounded-full border border-cyan-400/30" />
      <div className="pointer-events-none absolute -right-16 bottom-10 h-52 w-52 rounded-full border border-indigo-400/25" />

      <div className="relative mx-auto w-full max-w-6xl space-y-8">
        <header className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-700 pb-6">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-md border border-slate-600 bg-slate-900 text-sm font-semibold text-cyan-300">
              D2C
            </div>

            <div>
              <h2 className="text-lg font-semibold">Call Dashboard</h2>

              <p className="text-sm text-slate-300">
                Create or join a secure room
              </p>
            </div>
          </div>

          <button className="rounded-md border border-slate-600 bg-slate-900 px-4 py-2 text-sm text-slate-100 transition hover:border-slate-500 hover:bg-slate-800">
            Guest Session
          </button>
        </header>

        <section>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            Video Call Workspace
          </h1>

          <p className="mt-2 text-sm text-slate-300 sm:text-base">
            Real-time audio and video communication with WebRTC.
          </p>
        </section>

        <section className="grid gap-6 md:grid-cols-2">
          <div className="group rounded-xl border border-slate-700 bg-slate-900/90 p-8 shadow-lg shadow-black/20 transition duration-300 hover:-translate-y-1 hover:border-cyan-400/50">
            <h2 className="text-2xl font-semibold">Create Room</h2>

            <p className="mt-2 text-sm text-slate-300">
              Start a new video call room and invite others to join.
            </p>

            <button
              onClick={handleCreateRoom}
              disabled={connecting}
              className="mt-6 w-full rounded-md border border-cyan-300/80 bg-cyan-400 px-6 py-3 text-sm font-bold text-slate-950 transition hover:scale-[1.01] hover:bg-cyan-300 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {connecting && connectingAction === "create"
                ? "Connecting..."
                : "Create Room"}
            </button>
          </div>

          <div className="group rounded-xl border border-slate-700 bg-slate-900/90 p-8 shadow-lg shadow-black/20 transition duration-300 hover:-translate-y-1 hover:border-indigo-400/50">
            <h2 className="text-2xl font-semibold">Join Room</h2>

            <p className="mt-2 text-sm text-slate-300">
              Enter a room ID to join an existing call.
            </p>

            <div className="mt-6 flex flex-col gap-3 sm:flex-row">
              <input
                type="text"
                value={roomID}
                onChange={(e) => setRoomID(e.target.value)}
                placeholder="Enter room ID"
                className="flex-1 rounded-md border border-slate-600 bg-slate-800 px-4 py-3 text-slate-100 placeholder:text-slate-400 focus:border-indigo-400 focus:outline-none"
              />

              <button
                onClick={() => {
                  void handleJoinRoom();
                }}
                disabled={!roomID.trim() || connecting}
                className="rounded-md border border-indigo-300/70 bg-indigo-400 px-6 py-3 text-sm font-bold text-slate-950 transition hover:scale-[1.01] hover:bg-indigo-300 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {connecting && connectingAction === "join"
                  ? "Connecting..."
                  : "Join"}
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}