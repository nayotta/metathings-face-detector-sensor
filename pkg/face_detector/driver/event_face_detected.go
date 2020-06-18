package face_detector_driver

import "time"

type FaceDetectedImpl struct {
	timestamp time.Time
	face      []byte
	snapshot  []byte
}

func (i *FaceDetectedImpl) Type() string {
	return "FaceDetected"
}

func (i *FaceDetectedImpl) Timestamp() time.Time {
	return i.timestamp
}

func (i *FaceDetectedImpl) Face() []byte {
	return i.face
}

func (i *FaceDetectedImpl) Snapshot() []byte {
	return i.snapshot
}
