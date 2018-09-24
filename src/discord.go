package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// TODO: control everything with discord commands

var (
	Commands = map[string]func(s *discordgo.Session, m *discordgo.MessageCreate){
		"!add":        addFileCommand,
		"!remove":     removeCommand,
		"!skipnext":   skipCommand,
		"!unskipnext": unskipCommand,
		"!kill":       nextCommand,
		"!current":    currentCommand,
		"!search":     searchCommand,
	}
)

func NewDiscord(token string) *discordgo.Session {
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal(err)
	}
	discord.AddHandler(commandHandler)

	err = discord.Open()
	if err != nil {
		log.Fatal(err)
	}
	return discord
}

func commandHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if !isOwner(m.Author.ID) {
		return
	}
	if !strings.HasPrefix(m.Content, "!") {
		return
	}

	args := strings.SplitN(m.Content, " ", 2)
	if c, ok := Commands[args[0]]; ok {
		go c(s, m)
	}
}

func addFileCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	args := strings.SplitN(m.Content, " ", 2)
	if len(args) < 2 {
		return
	}

	file := ""
	err := filepath.Walk(config.Discord.MediaFolder, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		if strings.EqualFold(f.Name(), args[1]) {
			file = path
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		log.Println(err)
		return
	}
	if file != "" {
		strims.AddFile(file)
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Color:       0x7CFC00,
			Title:       "Added to queue",
			Description: filepath.Base(file),
			Fields: []*discordgo.MessageEmbedField{
				&discordgo.MessageEmbedField{
					Name:  "Position",
					Value: fmt.Sprintf("%d", len(strims.Queue)),
				},
			},
		})
		return
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Color:       0xFFA500,
		Description: fmt.Sprintf("couldn't find file for %s", args[1]),
	})
}

func nextCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	err := strims.CurrentStream.Kill()
	if err != nil {
		log.Println(err)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Color:       0xff0000,
		Description: fmt.Sprintf("Killed current stream %s", filepath.Base(strims.CurrentStream.File)),
	})
}

func skipCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	if len(strims.Queue) <= 0 {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Description: "Queue is empty",
		})
		return
	}
	strims.SkipNext = true
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Description: "Skipping next file " + filepath.Base(strims.Queue[0].File),
	})
}

func unskipCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	strims.SkipNext = false
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Description: "Unskipping next file",
	})
}

func removeCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	args := strings.SplitN(m.Content, " ", 2)
	if len(args) < 2 {
		return
	}

	if !containsFile(strims.Queue, args[1]) {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Description: fmt.Sprintf("%s not in Queue", args[1]),
		})
		return
	}

	for i, st := range strims.Queue {
		if strings.EqualFold(filepath.Base(st.File), args[1]) {
			s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
				Color:       0xff0000,
				Description: "Removed file " + filepath.Base(st.File),
			})
			strims.QueueMutex.Lock()
			copy(strims.Queue[i:], strims.Queue[i+1:])
			strims.Queue[len(strims.Queue)-1] = nil
			strims.Queue = strims.Queue[:len(strims.Queue)-1]
			strims.QueueMutex.Unlock()
		}
	}

}

func currentCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	if strims.CurrentStream == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Description: "Streaming nothing atm",
		})
		return
	}
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title: filepath.Base(strims.CurrentStream.File),
		Color: 0x7CFC00,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:  "Started",
				Value: strims.CurrentStream.StartTime.UTC().Format("15:04:05 MST"),
			},
		},
	})
}

func searchCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	args := strings.SplitN(m.Content, " ", 2)
	if len(args) < 2 {
		return
	}

	file := []string{}
	err := filepath.Walk(config.Discord.MediaFolder, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		if strings.Contains(strings.ToLower(f.Name()), strings.ToLower(args[1])) {
			file = append(file, f.Name())
		}
		return nil
	})
	if err != nil {
		log.Println(err)
		return
	}
	if len(file) == 0 {
		return
	}
	d := strings.Join(file, "\n")
	if len(d) > 2000 {
		d = d[:1997] + "..."
	}
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:       "Search",
		Color:       0x7CFC00,
		Description: d,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:  "Files",
				Value: fmt.Sprintf("%d", len(file)),
			},
		},
	})
}

func isOwner(id string) bool {
	return strings.EqualFold(config.Discord.OwnerID, id)
}
