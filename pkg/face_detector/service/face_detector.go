package face_detector_service

import (
	"bytes"
	"fmt"
	"io"

	"github.com/spf13/cast"

	driver "github.com/nayotta/metathings-sensor-face-detector/pkg/face_detector/driver"
	cfg_helper "github.com/nayotta/metathings/pkg/common/config"
	id_helper "github.com/nayotta/metathings/pkg/common/id"
	component "github.com/nayotta/metathings/pkg/component"
)

type FaceDetectorService struct {
	module        *component.Module
	face_detector driver.FaceDetector
	event_stream  *component.FrameStream
}

func (fds *FaceDetectorService) is_trigger_event() bool {
	return fds.event_stream != nil
}

func (fds *FaceDetectorService) startup() {
	logger := fds.module.Logger()

	knl := fds.module.Kernel()
	kc := knl.Config()
	drv, args, err := cfg_helper.ParseConfigOption("name", cast.ToStringMap(kc.Get("face_detector")),
		"logger", logger,
	)
	if err != nil {
		defer fds.module.Stop()
		logger.WithError(err).Errorf("failed to parse face detector config")
		return
	}

	ntf_flw_n := kc.GetString("notification.flow_name")
	if ntf_flw_n != "" {
		fds.event_stream, err = knl.NewFrameStream(ntf_flw_n)
		if err != nil {
			defer fds.module.Stop()
			logger.WithError(err).Errorf("failed to new frame stream")
			return
		}
	}

	fds.face_detector, err = driver.NewFaceDetector(drv, args...)
	if err != nil {
		defer fds.module.Stop()
		logger.WithError(err).Errorf("failed to new face detector")
		return
	}

	go fds.face_detector_loop()
}

func (fds *FaceDetectorService) face_detector_loop() {
	logger := fds.module.Logger()

	defer fds.module.Stop()
	for {
		select {
		case evt, ok := <-fds.face_detector.Detect():
			if !ok {
				logger.Warningf("face detector closed")
				return
			}

			fde := driver.ToFaceDetected(evt)
			if fde == nil {
				logger.Warningf("unexpected event type")
				continue
			}

			id := id_helper.NewId()
			ts := fde.Timestamp()
			face_path := fmt.Sprintf("/face/%04d/%02d/%02d/%v.jpg", ts.Year(), ts.Month(), ts.Day(), id)
			snapshot_path := fmt.Sprintf("/snapshot/%04d/%02d/%02d/%v.jpg", ts.Year(), ts.Month(), ts.Day(), id)

			err := fds.module.PutObjects(map[string]io.Reader{
				face_path:     bytes.NewReader(fde.Face()),
				snapshot_path: bytes.NewReader(fde.Snapshot()),
			})
			if err != nil {
				logger.WithError(err).Errorf("failed to put objects")
				return
			}

			if fds.is_trigger_event() {
				if err = fds.event_stream.Push(map[string]interface{}{
					"type": "FaceDetected",
					"module": map[string]interface{}{
						"name": fds.module.Name(),
					},
				}); err != nil {
					logger.WithError(err).Errorf("failed to push face detected event")
					return
				}
			}
		}
	}
}

func (fds *FaceDetectorService) InitModuleService(m *component.Module) error {
	fds.module = m

	go fds.startup()

	return nil
}
