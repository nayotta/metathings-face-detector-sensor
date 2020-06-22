package main

import (
	service "github.com/nayotta/metathings-sensor-face-detector/pkg/face_detector/service"
	component "github.com/nayotta/metathings/pkg/component"
)

func main() {
	mdl, err := component.NewModule("face-detector", new(service.FaceDetectorService))
	if err != nil {
		panic(err)
	}

	err = mdl.Launch()
	if err != nil {
		panic(err)
	}
}
