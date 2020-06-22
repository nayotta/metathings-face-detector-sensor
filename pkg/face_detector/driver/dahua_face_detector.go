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

type DahuaFaceDetectorOption struct {
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

func NewDahuaFaceDetectorOption() *DahuaFaceDetectorOption {
	opt := &DahuaFaceDetectorOption{}

	opt.Watchloop.Interval = 13 * time.Second
	opt.Mainloop.Timeout = 7 * time.Second
	opt.Fsnotifyloop.Timeout = 5 * time.Second

	return opt
}

type DahuaFaceDetector struct {
	opt           *DahuaFaceDetectorOption
	logger        logrus.FieldLogger
	watcher       *fsnotify.Watcher
	events        chan Event
	mainloop_chan chan string
	fsnotify_map  map[string]chan struct{}
	close_once    sync.Once
	closed        bool
}

func (dfd *DahuaFaceDetector) get_logger() logrus.FieldLogger {
	return dfd.logger
}

func (dfd *DahuaFaceDetector) Detect() <-chan Event {
	return dfd.events
}

func (dfd *DahuaFaceDetector) Close() {
	dfd.close_once.Do(func() {
		logger := dfd.get_logger()

		close(dfd.events)
		close(dfd.mainloop_chan)
		dfd.watcher.Close()
		dfd.closed = true

		logger.Debugf("face detector closed")
	})
}

func (dfd *DahuaFaceDetector) fsnotify_create_event_handler(evt fsnotify.Event) {
	fn := evt.Name
	sign := make(chan struct{})
	dfd.fsnotify_map[fn] = sign

	go dfd.fsnotify_loop(fn, sign)
}

func (dfd *DahuaFaceDetector) fsnotify_write_event_handler(evt fsnotify.Event) {
	fn := evt.Name

	logger := dfd.get_logger().WithField("file", fn)

	sign, ok := dfd.fsnotify_map[fn]
	if !ok {
		logger.Warningf("failed to get file sign channel")
		return
	}

	sign <- struct{}{}
}

func (dfd *DahuaFaceDetector) fsnotify_loop(fn string, sign chan struct{}) {
	logger := dfd.get_logger().WithFields(logrus.Fields{
		"file": fn,
		"#at":  "fsnotify_loop",
	})
	defer close(sign)
	defer delete(dfd.fsnotify_map, fn)
	for {
		select {
		case <-sign:
			logger.Debugf("file syncing")
		case <-time.After(dfd.opt.Fsnotifyloop.Timeout):
			dfd.mainloop_chan <- fn
			logger.Debugf("file done")
			return
		}
	}
}

func (dfd *DahuaFaceDetector) is_mfile(fn string) bool {
	return path.Ext(fn) == ".jpg" && strings.Contains(path.Base(fn), "[M]")
}

func (dfd *DahuaFaceDetector) is_rfile(fn string) bool {
	return path.Ext(fn) == ".jpg" && strings.Contains(path.Base(fn), "[R]")
}

func (dfd *DahuaFaceDetector) mainloop() {
	var fdi *FaceDetectedImpl
	logger := dfd.get_logger()

	defer dfd.Close()

	for !dfd.closed {
		select {
		case fn, ok := <-dfd.mainloop_chan:
			if !ok {
				return
			}

			// send old face detected event and create new one.
			if dfd.is_mfile(fn) {
				if fdi != nil {
					dfd.events <- fdi
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
			} else if dfd.is_rfile(fn) {
				if fdi != nil {
					buf, err := ioutil.ReadFile(fn)
					if err != nil {
						logger.WithError(err).Debugf("failed to read file")
						return
					}

					fdi.snapshot = buf
					dfd.events <- fdi
					fdi = nil
				}
			}
		case <-time.After(dfd.opt.Mainloop.Timeout):
			if fdi != nil {
				dfd.events <- fdi
				fdi = nil
			}
		}
	}
}

func (dfd *DahuaFaceDetector) watchloop() {
	logger := dfd.get_logger()

	defer dfd.Close()
	for !dfd.closed {
		select {
		case event, ok := <-dfd.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Create == fsnotify.Create {
				dfd.fsnotify_create_event_handler(event)
			} else if event.Op&fsnotify.Write == fsnotify.Write {
				dfd.fsnotify_write_event_handler(event)
			}
		case err, ok := <-dfd.watcher.Errors:
			if ok {
				logger.WithError(err).Warningf("receive fswatcher error")

			}
			return
		case <-time.After(dfd.opt.Watchloop.Interval):
			continue
		}
	}
}

func NewDahuaFaceDetector(args ...interface{}) (FaceDetector, error) {
	var err error
	var logger logrus.FieldLogger
	opt := NewDahuaFaceDetectorOption()

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

	dfd := &DahuaFaceDetector{
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
	register_face_detector_factory("dahua", NewDahuaFaceDetector)
}
