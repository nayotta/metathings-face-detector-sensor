package face_detector_driver

import (
	"sync"
	"time"
)

type Event interface {
	Type() string
	Timestamp() time.Time
}

type FaceDetected interface {
	Event

	Face() []byte
	Snapshot() []byte
}

type FaceDetector interface {
	Detect() <-chan Event
	Close()
}

type FaceDetectorFactory func(...interface{}) (FaceDetector, error)

var face_detector_factories map[string]FaceDetectorFactory
var face_detector_factories_once sync.Once

func register_face_detector_factory(name string, fty FaceDetectorFactory) {
	face_detector_factories_once.Do(func() {
		face_detector_factories[name] = fty
	})
}

func NewFaceDetector(name string, args ...interface{}) (FaceDetector, error) {
	fty, ok := face_detector_factories[name]
	if !ok {
		return nil, ErrUnsupportedFaceDetectorDriver
	}

	return fty(args...)
}
