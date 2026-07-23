package participant

type PublisherCallbacks struct {
	OnTrackPublished func(track *PublishedTrack, client *Client)
	// add more fields here later as new events show up — OnTrackUnpublished, etc.
}

type SubscriberCallbacks struct {
}
