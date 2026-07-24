export type CreateRoomResponse = {
    roomId : string,
    userId : string,
}

export type JoinRoomResponse = {
    roomId : string,
    userId : string,
}

export type PeerState = {
    userId: string,
    audioBool: boolean,
    videoBool: boolean,
}

export type ViewRoomResponse = {
    otherPeers: PeerState[],
}

export type LeaveRoomResponse = {
    message : string,
}

export type ErrorResponse = {
    message : string,
}