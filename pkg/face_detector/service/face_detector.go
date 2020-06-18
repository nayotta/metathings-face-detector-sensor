package face_detector_service

import (
	component "github.com/nayotta/metathings/pkg/component"
)

type FaceDetectorService struct {
	module *component.Module
}

func (s *FaceDetectorService) InitModuleService(m *component.Module) error {
	s.module = module

	return nil
}
