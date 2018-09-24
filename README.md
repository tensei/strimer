# Strimer

## Setup

    cp example.config.json config.json

Edit docker-compose.yml

    version: "3"
    services:
    base:
        build: .
        network_mode: "host"
        restart: unless-stopped
        volumes:
        - /path/to/your/media/folder/here/:/path/to/your/media/folder/here/

same path in config.json

    {
        "discord": {
            "owner_id": "<your discord id>",
            "token": "<your bot token>",
            "media_folder": "/path/to/your/media/folder/here/"
        },
        "stream": {
            "ingest": "rtmp://fra-ingest.angelthump.com:1935/live/<your streamkey>",
            "subtitles": false
        }
    }

## Docker

    docker-compose up -d
