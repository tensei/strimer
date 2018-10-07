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
        - /path/to/your/media/folder/here/:/media/

Edite config.json

Don't enable bumps unless you know what you're doing

    {
        "discord": {
            "owner_id": "105739663192363008",
            "token": "<bot token>",
            "media_folder": "/media/"
        },
        "stream": {
            "ingest": "rtmp://fra-ingest.angelthump.com:1935/live/xxx",
            "subtitles": true,
            "bumps": false
        },
        "angelthump": {
            "update_title": false,
            "username": "",
            "password": ""
        }
    }

## Docker

    docker-compose build
    docker-compose up -d
