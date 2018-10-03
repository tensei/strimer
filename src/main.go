package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
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
	File          string
	audiotrack    string
	subtitletrack string
	ispreroll     bool

	cmd *exec.Cmd

	streaming bool
	Restart   bool

	StartTime time.Time
	EndTime   time.Time
}

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	config = LoadConfig()
	if config.Angelthump.UpdateTitle {
		err := config.Angelthump.Login()
		if err != nil {
			log.Fatalf("error logging in Angelthump: %v", err)
		}
	}
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

func (s *Strims) AddFile(file, audiotrack, subtitletrack string, ispreroll bool) {
	if audiotrack == "" {
		audiotrack = "0"
	}
	if subtitletrack == "" {
		subtitletrack = "0"
	}
	s.Queue = append(s.Queue, &Stream{
		File:          file,
		audiotrack:    audiotrack,
		subtitletrack: subtitletrack,
		ispreroll:     ispreroll,
		cmd:           createCmd(file, audiotrack, subtitletrack),
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
		if !stream.ispreroll {
			err := config.Angelthump.ChangeTitle(filepath.Base(stream.File))
			if err != nil {
				log.Println(err)
			}
		}

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

		if stream.Restart {
			stream.Restart = false
			continue
		}

		s.QueueMutex.Lock()
		s.Queue = s.Queue[1:]
		s.QueueMutex.Unlock()
		if stream.ispreroll {
			os.Remove(stream.File)
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

func createCmd(file, a, s string) *exec.Cmd {
	subarg := ""
	if s != "-1" && config.Stream.Subtitles {
		streams := getStreams(file)
		if strings.Contains(streams, "hdmv_pgs_subtitle") || strings.Contains(streams, "dvd_subtitle") {
			subarg = fmt.Sprintf(`-tune animation -filter_complex "[0:v][0:s:%s]overlay[v]" -map "[v]" -map 0:a:%s `, s, a)
		} else {
			// title := fmt.Sprintf(`-vf "drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf:text='%s':fontcolor=white:x=(w-text_w)/2:y=16:fontsize=12" `, filepath.Base(file))
			subarg = fmt.Sprintf("-tune animation -vf subtitles='%s':si=%s -map 0:0 -map 0:a:%s ", regexp.QuoteMeta(file), s, a)
		}
	}
	// TODO video, audio and subtitle track stuff
	args := []string{
		"bash", "-c",
		fmt.Sprintf("ffmpeg -re -i '%s' %s-c:v libx264 -pix_fmt yuv420p -preset faster -b:v 3500k -maxrate 3500k -x264-params keyint=60 -c:a aac -strict -2 -ar 44100 -b:a 160k -ac 2 -bufsize 7000k -f flv %s", file, subarg, config.Stream.Ingest),
	}
	log.Println(strings.Join(args, " "))
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

func getStreams(file string) string {
	args := []string{
		"bash", "-c",
		fmt.Sprintf("ffmpeg -i '%s' 2>&1 | grep 'Stream #'", file),
	}
	log.Println(strings.Join(args, " "))
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err.Error()
	}
	return string(out)
}

func createPreroll(file string) (string, error) {
	ass, err := createPrerollAss(file)
	if err != nil {
		return "", err
	}
	defer os.Remove(strings.Replace(ass, "\\", "/", -1))
	m := md5.New()
	io.WriteString(m, fmt.Sprintf("%s%d", file, time.Now().Unix()))
	args := []string{
		"bash",
		"-c",
		fmt.Sprintf("./data/clip.sh %s %x", strings.Replace(ass, "\\", "/", -1), m.Sum(nil)),
	}
	cmd := exec.Command(args[0], args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s, %v", out, err)
	}
	return fmt.Sprintf("./data/tempprerolls/%x.mp4", m.Sum(nil)), nil
}

func createPrerollAss(name string) (string, error) {
	asst, err := ioutil.ReadFile("./data/template.ass")
	if err != nil {
		return "", err
	}
	asst = bytes.Replace(asst, []byte("[[file]]"), []byte(name), -1)
	temp, err := ioutil.TempFile("./data/tempasses", "*.ass")
	if err != nil {
		return "", err
	}
	temp.Write(asst)
	temp.Close()
	return temp.Name(), nil
}
