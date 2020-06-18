package main

import (
	"crypto/md5"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	fddrv "github.com/nayotta/metathings-sensor-face-detector/pkg/face_detector/driver"
)

var (
	path string
)

func main() {
	pflag.StringVarP(&path, "path", "p", "", "watch path")
	pflag.Parse()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	fd, err := fddrv.NewFaceDetector(
		"dahua",
		"logger", logrus.New(),
		"path", path,
	)
	if err != nil {
		panic(err)
	}

	for {
		select {
		case evt := <-fd.Detect():
			switch evt.Type() {
			case "FaceDetected":
				fde, ok := evt.(fddrv.FaceDetected)
				if !ok {
					panic("failed to convert event to FaceDetected")
				}

				h := md5.New()
				h.Write(fde.Face())
				md5_face := h.Sum(nil)

				h = md5.New()
				h.Write(fde.Snapshot())
				md5_snapshot := h.Sum(nil)

				fmt.Printf("ts=%v, md5(evt.Face())=%x, md5(evt.Snapshot())=%x\n", fde.Timestamp(), md5_face, md5_snapshot)
			}
		}
	}
}
