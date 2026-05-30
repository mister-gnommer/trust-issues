package store

import "time"

type Artist struct {
	ID   string
	Name string
}

type Track struct {
	ID         string
	Name       string
	DurationMS int64
	ArtistIDs  []string
}

type Snapshot struct {
	ID           string
	UserID       string
	PlaylistID   string
	PlaylistName string
	CapturedAt   time.Time
}

type SnapshotTrack struct {
	TrackID  string
	Position int
}

type Play struct {
	ID                    int64
	UserID                string
	TrackID               string
	PlaylistID            *string
	PlaylistSnapshotID    *string
	ShuffleState          bool
	SmartShuffle          bool
	PlayedAt              time.Time
	EndedAt               *time.Time
	ProgressMSAtDetection int64
	Skipped               *bool
}
