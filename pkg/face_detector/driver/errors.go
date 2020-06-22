package face_detector_driver

import "errors"

var (
	ErrUnexpectedEvent               = errors.New("unexpected event")
	ErrUnsupportedFaceDetectorDriver = errors.New("unsupported face detector driver")
)
