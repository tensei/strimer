package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// TODO: control everything with discord commands

type CommandContext struct {
	s     *discordgo.Session
	m     *discordgo.MessageCreate
	regex string
}

var (
	Commands = map[string]func(ctx CommandContext){
		`^!add(:a\d+)?(:s\d+)?$`: addFileCommand,
		"^!remove$":              removeCommand,
		"^!skipnext$":            skipCommand,
		"^!unskipnext$":          unskipCommand,
		"^!kill$":                nextCommand,
		"^!current$":             currentCommand,
		"^!search$":              searchCommand,
		"^!filestreams$":         showFileStreamsCommand,
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
	for c, f := range Commands {
		if ok, _ := regexp.MatchString(c, args[0]); ok {
			go f(CommandContext{s, m, c})
			return
		}
	}
}

func addFileCommand(ctx CommandContext) {
	args := strings.SplitN(ctx.m.Content, " ", 2)
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
		m := regexp.MustCompile(ctx.regex).FindAllStringSubmatch(args[0], -1)
		a, s := "0", "0"
		if len(m) > 0 {
			for _, arg := range m[0][1:] {
				if arg == "" {
					continue
				}
				log.Println(arg, "jj")
				if arg[1] == 'a' {
					a = arg[2:]
				}
				if arg[1] == 's' {
					s = arg[2:]
				}
			}
		}
		strims.AddFile(file, a, s)
		ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
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

	ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
		Color:       0xFFA500,
		Description: fmt.Sprintf("couldn't find file for %s", args[1]),
	})
}

func nextCommand(ctx CommandContext) {
	err := strims.CurrentStream.Kill()
	if err != nil {
		log.Println(err)
	}

	ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
		Color:       0xff0000,
		Description: fmt.Sprintf("Killed current stream %s", filepath.Base(strims.CurrentStream.File)),
	})
}

func skipCommand(ctx CommandContext) {
	if len(strims.Queue) <= 0 {
		ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
			Description: "Queue is empty",
		})
		return
	}
	strims.SkipNext = true
	ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
		Description: "Skipping next file " + filepath.Base(strims.Queue[0].File),
	})
}

func unskipCommand(ctx CommandContext) {
	strims.SkipNext = false
	ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
		Description: "Unskipping next file",
	})
}

func removeCommand(ctx CommandContext) {
	args := strings.SplitN(ctx.m.Content, " ", 2)
	if len(args) < 2 {
		return
	}

	if !containsFile(strims.Queue, args[1]) {
		ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
			Description: fmt.Sprintf("%s not in Queue", args[1]),
		})
		return
	}

	strims.QueueMutex.Lock()
	defer strims.QueueMutex.Unlock()

	for i, st := range strims.Queue {
		if st == nil {
			continue
		}
		if strings.EqualFold(filepath.Base(st.File), args[1]) {
			ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
				Color:       0xff0000,
				Description: "Removed file " + filepath.Base(st.File),
			})
			copy(strims.Queue[i:], strims.Queue[i+1:])
			strims.Queue[len(strims.Queue)-1] = nil
			strims.Queue = strims.Queue[:len(strims.Queue)-1]
		}
	}

}

func currentCommand(ctx CommandContext) {
	if strims.CurrentStream == nil {
		ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
			Description: "Streaming nothing atm",
		})
		return
	}
	ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
		Title: filepath.Base(strims.CurrentStream.File),
		Color: 0x7CFC00,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:  "Started",
				Value: strims.CurrentStream.StartTime.UTC().Format("15:04:05 MST"),
			},
			&discordgo.MessageEmbedField{
				Name:  "Current time",
				Value: time.Now().UTC().Format("15:04:05 MST"),
			},
		},
	})
}

func searchCommand(ctx CommandContext) {
	args := strings.SplitN(ctx.m.Content, " ", 2)
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
	ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
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

func showFileStreamsCommand(ctx CommandContext) {
	args := strings.SplitN(ctx.m.Content, " ", 2)
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
		ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
			Color:       0x7CFC00,
			Title:       "File Streams",
			Description: getStreams(file),
		})
		return
	}

	ctx.s.ChannelMessageSendEmbed(ctx.m.ChannelID, &discordgo.MessageEmbed{
		Color:       0xFFA500,
		Description: fmt.Sprintf("couldn't find file for %s", args[1]),
	})
}

func isOwner(id string) bool {
	return strings.EqualFold(config.Discord.OwnerID, id)
}
