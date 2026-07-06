export type OfferMessage = {
  type: "offer";
  sdp: RTCSessionDescriptionInit;
};

export type PopulateRoomMessage = {
  type: "populate-room";
  roomId: string;
  userId: string;
};

export type IceCandidateMessage = {
  type: "ice-candidate";
  iceCandidate: RTCIceCandidateInit;
};

export type AudioToggle = {
  type : "audio-toggle",
  muted : boolean
}

export type VideoToggle = {
  type : "video-toggle",
  muted : boolean
}

export type ClientToServerMessage =
  | OfferMessage
  | PopulateRoomMessage
  | SubscriberAnswerMessage
  | SubscriberICEMessage
  | PeerLeftMessage
  | IceCandidateMessage
  | AudioToggle
  | VideoToggle;

export type AnswerMessage = {
  type: "answer";
  sdp: RTCSessionDescriptionInit;
};

export type SubscriberAnswerMessage = {
  type: "subscriber-answer";
  sdp: RTCSessionDescriptionInit;
};

export type SubscriberOfferMessage = {
  type: "subscriber-offer";
  sdp: RTCSessionDescriptionInit;
};

export type SubscriberICEMessage = {
  type: "subscriber-ice-candidate";
  iceCandidate: RTCIceCandidateInit;
};

export type PeerJoinedMessage = {
  type: "peer-joined";
  userId: string;
};

export type PeerLeftMessage = {
  type: "peer-left";
  roomId : string;
  userId: string;
};

export type MediaPublishedMessage = {
  type: "media-published";
  mid: string;
  publisher: string;
};

export type IncomingIceCandidateMessage = {
  type: "ice-candidate";
  iceCandidate: RTCIceCandidateInit;
};

type IncomingAudioToggle = {
    type: "audio-toggle";
    userId: string;
    muted: boolean;
}

type IncomingVideoToggle = {
    type: "video-toggle";
    userId: string;
    muted: boolean;
}

export type ServerToClientMessage =
  | AnswerMessage
  | SubscriberOfferMessage
  | SubscriberICEMessage
  | PeerJoinedMessage
  | PeerLeftMessage
  | MediaPublishedMessage
  | IncomingIceCandidateMessage
  | IncomingAudioToggle 
  | IncomingVideoToggle;

export type WSMessage = ClientToServerMessage | ServerToClientMessage;
