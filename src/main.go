package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	config Config
	strims *Strims
)

type Strims struct {
	Queue      []*Stream
	QueueMutex sync.Mutex

	CurrentIndex  int
	CurrentStream *Stream
	StopStream    chan struct{}
	SkipNext      bool

	StartTime time.Time
	EndTime   time.Time
}

type Stream struct {
	File string
	cmd  *exec.Cmd

	streaming bool

	StartTime time.Time
	EndTime   time.Time
}

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	config = LoadConfig()
	strims = NewStrims()

	d := NewDiscord(config.Discord.Token)
	defer d.Close()

	go strims.StartStreaming()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint

}

func NewStrims() *Strims {
	return &Strims{
		Queue:        []*Stream{},
		StartTime:    time.Now(),
		CurrentIndex: -1,
		StopStream:   make(chan struct{}, 1),
	}
}

func (s *Strims) AddFile(file string) {
	s.Queue = append(s.Queue, &Stream{
		File: file,
		cmd:  createCmd(file),
	})
}

func (s *Strims) StartStreaming() {

	log.Printf("%d files. starting in 10 seconds...", len(s.Queue))
	time.Sleep(time.Second * 10)
	done := make(chan error, 1)

	for {

		if len(s.Queue) == 0 {
			time.Sleep(time.Second * 5)
			continue
		}

		stream := s.Queue[0]

		s.QueueMutex.Lock()
		s.Queue = s.Queue[1:]
		s.QueueMutex.Unlock()

		if s.SkipNext {
			log.Printf("SKIPPING %s", filepath.Base(stream.File))
			s.SkipNext = false
			s.CurrentStream = nil
			continue
		}

		s.CurrentStream = stream
		log.Printf("STARTING %s", filepath.Base(stream.File))
		if err := stream.Start(); err != nil {
			log.Println(err)
			continue
		}
		go stream.Wait(done)

		select {
		case err := <-done:
			if err != nil {
				log.Println(err)
			}
		case <-s.StopStream:
			if err := stream.Kill(); err != nil {
				log.Println(err)
			}
		}
		log.Printf("%d more in queue", len(s.Queue))
	}
}

func (s *Stream) Start() error {
	s.StartTime = time.Now()
	s.streaming = true
	if err := s.cmd.Start(); err != nil {
		return err
	}
	return nil
}

func (s *Stream) Kill() error {
	s.streaming = false
	if err := s.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %v", err)
	}
	return nil
}

func (s *Stream) Wait(done chan error) {
	done <- s.cmd.Wait()
	s.EndTime = time.Now()
	s.streaming = false
}

func createCmd(file string) *exec.Cmd {
	subarg := ""
	if config.Stream.Subtitles {
		subarg = fmt.Sprintf("-vf subtitles='%s' ", regexp.QuoteMeta(file))
	}
	args := []string{
		"bash", "-c",
		fmt.Sprintf("ffmpeg -re -i '%s' %s-c:v libx264 -pix_fmt yuv420p -preset faster -b:v 3500k -maxrate 3500k -x264-params keyint=60 -c:a aac -strict -2 -ar 44100 -b:a 160k -ac 2 -bufsize 7000k -f flv %s", file, subarg, config.Stream.Ingest),
	}
	return exec.Command(args[0], args[1:]...)
}

func containsFile(streams []*Stream, file string) bool {
	for _, s := range streams {
		if strings.EqualFold(filepath.Base(s.File), file) {
			return true
		}
	}
	return false
}
