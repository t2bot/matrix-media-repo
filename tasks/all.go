package tasks

func StartAll() {
	StartRemoteMediaPurgeRecurring()
	StartThumbnailPurgeRecurring()
	StartPreviewsPurgeRecurring()
}

func StopAll() {
	StopRemoteMediaPurgeRecurring()
	StopThumbnailPurgeRecurring()
	StopPreviewsPurgeRecurring()
}
