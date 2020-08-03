package face_detector_driver

import (
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	opt_helper "github.com/nayotta/metathings/pkg/common/option"
)

type HikvisionFaceDetectorOption struct {
	Path      string
	Watchloop struct {
		Interval time.Duration
	}
	Mainloop struct {
		Timeout time.Duration
	}
	Fsnotifyloop struct {
		Timeout time.Duration
	}
}

func NewHikvisionFaceDetectorOption() *HikvisionFaceDetectorOption {
	opt := &HikvisionFaceDetectorOption{}

	opt.Watchloop.Interval = 13 * time.Second
	opt.Mainloop.Timeout = 7 * time.Second
	opt.Fsnotifyloop.Timeout = 5 * time.Second

	return opt
}

type HikvisionFaceDetector struct {
	opt           *HikvisionFaceDetectorOption
	logger        logrus.FieldLogger
	watcher       *fsnotify.Watcher
	events        chan Event
	mainloop_chan chan string
	fsnotify_map  map[string]chan struct{}
	close_once    sync.Once
	closed        bool
}

func (hfd *HikvisionFaceDetector) get_logger() logrus.FieldLogger {
	return hfd.logger
}

func (hfd *HikvisionFaceDetector) Detect() <-chan Event {
	return hfd.events
}

func (hfd *HikvisionFaceDetector) Close() {
	hfd.close_once.Do(func() {
		logger := hfd.get_logger()

		close(hfd.events)
		close(hfd.mainloop_chan)
		hfd.watcher.Close()
		hfd.closed = true

		logger.Debugf("face detector closed")
	})
}

func (hfd *HikvisionFaceDetector) fsnotify_create_event_handler(evt fsnotify.Event) {
	fn := evt.Name
	sign := make(chan struct{})
	hfd.fsnotify_map[fn] = sign

	go hfd.fsnotify_loop(fn, sign)
}

func (hfd *HikvisionFaceDetector) fsnotify_write_event_handler(evt fsnotify.Event) {
	fn := evt.Name

	logger := hfd.get_logger().WithField("file", fn)

	sign, ok := hfd.fsnotify_map[fn]
	if !ok {
		logger.Warningf("failed to get file sign channel")
		return
	}

	sign <- struct{}{}
}

func (hfd *HikvisionFaceDetector) fsnotify_loop(fn string, sign chan struct{}) {
	logger := hfd.get_logger().WithFields(logrus.Fields{
		"file": fn,
		"#at":  "fsnotify_loop",
	})
	defer close(sign)
	defer delete(hfd.fsnotify_map, fn)
	for {
		select {
		case <-sign:
			logger.Debugf("file syncing")
		case <-time.After(hfd.opt.Fsnotifyloop.Timeout):
			hfd.mainloop_chan <- fn
			logger.Debugf("file done")
			return
		}
	}
}

func (hfd *HikvisionFaceDetector) is_mfile(fn string) bool {
	return path.Ext(fn) == ".jpg" && strings.Contains(path.Base(fn), "FACE_SNAP")
}

func (hfd *HikvisionFaceDetector) is_rfile(fn string) bool {
	return path.Ext(fn) == ".jpg" && strings.Contains(path.Base(fn), "FACE_BACKGROUND")
}

func (hfd *HikvisionFaceDetector) mainloop() {
	var fdi *FaceDetectedImpl
	logger := hfd.get_logger()

	defer hfd.Close()

	for !hfd.closed {
		select {
		case fn, ok := <-hfd.mainloop_chan:
			if !ok {
				return
			}

			// send old face detected event and create new one.
			if hfd.is_mfile(fn) {
				if fdi != nil {
					hfd.events <- fdi
					fdi = nil
				}

				buf, err := ioutil.ReadFile(fn)
				if err != nil {
					logger.WithError(err).Debugf("failed to read file")
					return
				}

				fdi = &FaceDetectedImpl{
					timestamp: time.Now(),
					face:      buf,
				}
			} else if hfd.is_rfile(fn) {
				if fdi != nil {
					buf, err := ioutil.ReadFile(fn)
					if err != nil {
						logger.WithError(err).Debugf("failed to read file")
						return
					}

					fdi.snapshot = buf
					hfd.events <- fdi
					fdi = nil
				}
			}
		case <-time.After(hfd.opt.Mainloop.Timeout):
			if fdi != nil {
				hfd.events <- fdi
				fdi = nil
			}
		}
	}
}

func (hfd *HikvisionFaceDetector) watchloop() {
	logger := hfd.get_logger()

	defer hfd.Close()
	for !hfd.closed {
		select {
		case event, ok := <-hfd.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Create == fsnotify.Create {
				hfd.fsnotify_create_event_handler(event)
			} else if event.Op&fsnotify.Write == fsnotify.Write {
				hfd.fsnotify_write_event_handler(event)
			}
		case err, ok := <-hfd.watcher.Errors:
			if ok {
				logger.WithError(err).Warningf("receive fswatcher error")

			}
			return
		case <-time.After(hfd.opt.Watchloop.Interval):
			continue
		}
	}
}

func NewHikvisionFaceDetector(args ...interface{}) (FaceDetector, error) {
	var err error
	var logger logrus.FieldLogger
	opt := NewHikvisionFaceDetectorOption()

	if err = opt_helper.Setopt(map[string]func(string, interface{}) error{
		"path":                 opt_helper.ToString(&opt.Path),
		"watchloop_interval":   opt_helper.ToDuration(&opt.Watchloop.Interval),
		"mainloop_timeout":     opt_helper.ToDuration(&opt.Mainloop.Timeout),
		"fsnotifyloop_timeout": opt_helper.ToDuration(&opt.Fsnotifyloop.Timeout),
		"logger":               opt_helper.ToLogger(&logger),
	})(args...); err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(opt.Path)
	if err != nil {
		return nil, err
	}

	dfd := &HikvisionFaceDetector{
		opt:           opt,
		logger:        logger,
		watcher:       watcher,
		events:        make(chan Event),
		mainloop_chan: make(chan string),
		fsnotify_map:  make(map[string]chan struct{}),
	}

	go dfd.watchloop()
	go dfd.mainloop()

	return dfd, nil
}

func init() {
	register_face_detector_factory("hikvision", NewHikvisionFaceDetector)
}
