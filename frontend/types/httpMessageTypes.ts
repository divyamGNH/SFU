export type CreateRoomResponse = {
    roomId : string,
    userId : string,
}

export type JoinRoomResponse = {
    roomId : string,
    userId : string,
}

export type ViewRoomResponse = {
    otherPeers : string[],
}

export type LeaveRoomResponse = {
    message : string,
}

export type ErrorResponse = {
    message : string,
}