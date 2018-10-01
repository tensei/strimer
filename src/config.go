package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Config struct {
	Discord struct {
		OwnerID     string `json:"owner_id"`
		Token       string `json:"token"`
		MediaFolder string `json:"media_folder"`
	} `json:"discord"`
	Stream struct {
		Ingest    string `json:"ingest"`
		Subtitles bool   `json:"subtitles"`
		Bumps     bool   `json:"bumps"`
	} `json:"stream"`
	Angelthump Angelthump `json:"angelthump"`
}

func LoadConfig() Config {
	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	var cfg Config
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}
